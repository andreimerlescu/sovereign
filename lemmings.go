package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// runLemmings deploys and executes lemmings on each planet concurrently.
// Planets with status PlanetStatusLost are skipped.
// A planet that fails to run lemmings within cfg.Until plus cfg.Ramp
// plus a five minute collection overhead is marked PlanetStatusLost.
// Lost planets during this phase are non-fatal -- collection continues
// on remaining planets.
func (s *Sovereign) runLemmings() error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, planet := range s.planets {
		planet := planet
		if planet.Status == PlanetStatusLost {
			fmt.Printf("  sovereign: skipping planet %d -- lost during provisioning\n",
				planet.ID)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.runLemmingsOnPlanet(planet); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	for _, err := range errs {
		fmt.Printf("  sovereign: warning: lemmings run error: %v\n", err)
	}

	return nil
}

// runLemmingsOnPlanet copies the lemmings config to the planet via SCP
// and executes lemmings on the remote instance via SSH. It blocks until
// lemmings completes or the run deadline is exceeded.
func (s *Sovereign) runLemmingsOnPlanet(planet *Planet) error {
	fmt.Printf("  sovereign: starting lemmings on planet %d (%s)...\n",
		planet.ID, planet.PublicIPv4)

	s.ledger.Append(LedgerEvent{
		PlanetID:   planet.ID,
		EventType:  LedgerEventLemmingsStart,
		PublicIPv4: planet.PublicIPv4,
	})

	// copy plan file if configured
	if s.cfg.Plan != "" {
		if err := s.copyPlanFile(planet); err != nil {
			planet.mu.Lock()
			planet.Status = PlanetStatusLost
			planet.Error = err
			planet.mu.Unlock()
			s.ledger.Append(LedgerEvent{
				PlanetID:  planet.ID,
				EventType: LedgerEventLemmingsFailed,
				Error:     err.Error(),
			})
			return fmt.Errorf("planet %d: copy plan file: %w", planet.ID, err)
		}
	}

	// build the run deadline
	deadline := s.cfg.Ramp + s.cfg.Until + 5*time.Minute
	ctx, cancel := context.WithTimeout(s.ctx, deadline)
	defer cancel()

	// execute lemmings on the remote instance via SSH
	cmd := exec.CommandContext(ctx,
		"ssh",
		"-i", s.planetSSHKeyPath(planet),
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=30",
		fmt.Sprintf("%s@%s", planet.RemoteUser, planet.PublicIPv4),
		s.buildLemmingsCommand(planet),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		planet.mu.Lock()
		planet.Status = PlanetStatusLost
		planet.Error = err
		planet.mu.Unlock()
		s.ledger.Append(LedgerEvent{
			PlanetID:  planet.ID,
			EventType: LedgerEventLemmingsFailed,
			Error:     err.Error(),
		})
		return fmt.Errorf("planet %d: lemmings run failed: %w", planet.ID, err)
	}

	s.ledger.Append(LedgerEvent{
		PlanetID:  planet.ID,
		EventType: LedgerEventLemmingsComplete,
	})

	fmt.Printf("  sovereign: planet %d lemmings run complete\n", planet.ID)
	return nil
}

// collectResults collects lemmings report files from each planet
// via SCP as each one completes. Collection runs concurrently across
// all non-lost planets. A collection failure marks the planet lost
// but does not block collection from other planets.
func (s *Sovereign) collectResults() error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, planet := range s.planets {
		planet := planet
		if planet.Status == PlanetStatusLost {
			fmt.Printf("  sovereign: skipping collection for planet %d -- lost\n",
				planet.ID)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.collectFromPlanet(planet); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	for _, err := range errs {
		fmt.Printf("  sovereign: warning: collection error: %v\n", err)
	}

	return nil
}

// collectFromPlanet SCPs the lemmings report files from a planet
// back to the sovereign working directory.
func (s *Sovereign) collectFromPlanet(planet *Planet) error {
	planet.mu.Lock()
	planet.Status = PlanetStatusCollecting
	planet.mu.Unlock()

	fmt.Printf("  sovereign: collecting results from planet %d (%s)...\n",
		planet.ID, planet.PublicIPv4)

	s.ledger.Append(LedgerEvent{
		PlanetID:   planet.ID,
		EventType:  LedgerEventCollectStart,
		PublicIPv4: planet.PublicIPv4,
	})

	localDir := s.planetReportDir(planet)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("planet %d: create report dir: %w", planet.ID, err)
	}

	cmd := exec.CommandContext(s.ctx,
		"scp",
		"-i", s.planetSSHKeyPath(planet),
		"-o", "StrictHostKeyChecking=no",
		"-r",
		fmt.Sprintf("%s@%s:~/lemmings/",
			planet.RemoteUser, planet.PublicIPv4),
		localDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		planet.mu.Lock()
		planet.Status = PlanetStatusLost
		planet.Error = err
		planet.mu.Unlock()
		s.ledger.Append(LedgerEvent{
			PlanetID:  planet.ID,
			EventType: LedgerEventCollectFailed,
			Error:     err.Error(),
		})
		return fmt.Errorf("planet %d: scp collect failed: %w", planet.ID, err)
	}

	planet.mu.Lock()
	planet.Status = PlanetStatusDone
	planet.CompletedAt = time.Now()
	planet.ReportPath = localDir
	planet.mu.Unlock()

	s.ledger.Append(LedgerEvent{
		PlanetID:  planet.ID,
		EventType: LedgerEventCollectComplete,
	})

	fmt.Printf("  sovereign: planet %d results collected to %s\n",
		planet.ID, localDir)

	return nil
}

// buildLemmingsCommand constructs the lemmings CLI invocation string
// for execution on a remote planet via SSH.
func (s *Sovereign) buildLemmingsCommand(planet *Planet) string {
	cmd := fmt.Sprintf(
		"lemmings -hit %s -terrain %d -pack %d -until %s -ramp %s -tty=false -save-to ~/lemmings",
		s.cfg.Hit,
		s.cfg.Terrain,
		s.cfg.Pack,
		s.cfg.Until,
		s.cfg.Ramp,
	)
	if s.cfg.Plan != "" {
		cmd += fmt.Sprintf(" -plan /home/%s/lemmings.plan.yml", planet.RemoteUser)
	}
	return cmd
}

// copyPlanFile SCPs the local lemmings plan YAML file to the planet.
func (s *Sovereign) copyPlanFile(planet *Planet) error {
	if s.cfg.Plan == "" {
		return nil
	}

	cmd := exec.CommandContext(s.ctx,
		"scp",
		"-i", s.planetSSHKeyPath(planet),
		"-o", "StrictHostKeyChecking=no",
		s.cfg.Plan,
		fmt.Sprintf("%s@%s:/home/%s/lemmings.plan.yml",
			planet.RemoteUser, planet.PublicIPv4, planet.RemoteUser),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("scp plan file to planet %d: %w", planet.ID, err)
	}

	return nil
}

// completedPlanets returns the subset of planets that reached
// PlanetStatusDone with a non-empty ReportPath.
func (s *Sovereign) completedPlanets() []*Planet {
	var completed []*Planet
	for _, p := range s.planets {
		if p.Status == PlanetStatusDone && p.ReportPath != "" {
			completed = append(completed, p)
		}
	}
	return completed
}

// planetReportDir returns the local directory where a planet's
// collected lemmings reports are stored.
func (s *Sovereign) planetReportDir(planet *Planet) string {
	return filepath.Join(s.planetWorkDir(planet), "reports")
}

// planetSSHKeyPath returns the SSH private key path for a planet.
// Matches the SSHKeyPath set in hclSpec so the same key terraform
// used to provision the instance is used for SSH access.
func (s *Sovereign) planetSSHKeyPath(planet *Planet) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "sovereign-planet.pem")
}
