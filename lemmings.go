package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/events"
)

// ErrAllPlanetsLost is returned by runLemmings when every planet
// was marked lost during execution, making collection and reporting
// impossible. The caller should still attempt destroyTerrain to clean
// up any instances that were provisioned before failure.
var ErrAllPlanetsLost = fmt.Errorf("all planets lost during lemmings execution")

// runLemmings deploys and executes lemmings on each planet concurrently.
// Planets with status PlanetStatusLost are skipped.
//
// Returns ErrAllPlanetsLost when no planet survived execution. Individual
// planet failures are logged as warnings but do not stop execution on
// surviving planets.
func (s *Sovereign) runLemmings() error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, planet := range s.planets {
		planet := planet
		if planet.Status == PlanetStatusLost {
			fmt.Fprintf(s.out, "  sovereign: skipping planet %d -- lost during provisioning\n",
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
		fmt.Fprintf(s.out, "  sovereign: warning: lemmings run error: %v\n", err)
	}

	// If every planet is now lost, report it so Run can abort cleanly
	// rather than attempting collection on an empty set.
	for _, p := range s.planets {
		if p.Status != PlanetStatusLost {
			return nil
		}
	}
	return ErrAllPlanetsLost
}

// runLemmingsOnPlanet copies the plan file if configured and executes
// lemmings on the remote instance via SSH through Commander.
func (s *Sovereign) runLemmingsOnPlanet(planet *Planet) error {
	fmt.Fprintf(s.out, "  sovereign: starting lemmings on planet %d (%s)...\n",
		planet.ID, planet.PublicIPv4)

	s.bus.Emit(events.Event{
		Kind:       events.KindLemmingsStart,
		PlanetID:   planet.ID,
		Provider:   s.provider.Name(),
		Region:     planet.Region,
		PublicIPv4: planet.PublicIPv4,
	})

	if s.cfg.Plan != "" {
		if err := s.copyPlanFile(planet); err != nil {
			planet.mu.Lock()
			planet.Status = PlanetStatusLost
			planet.Error = err
			planet.mu.Unlock()
			s.bus.Emit(events.Event{
				Kind:     events.KindLemmingsFailed,
				PlanetID: planet.ID,
				Provider: s.provider.Name(),
				Region:   planet.Region,
				Err:      err,
			})
			return fmt.Errorf("planet %d: copy plan file: %w", planet.ID, err)
		}
	}

	deadline := s.cfg.Ramp + s.cfg.Until + 5*time.Minute
	ctx, cancel := context.WithTimeout(s.ctx, deadline)
	defer cancel()

	sshArgs := s.sshArgs(planet, s.buildLemmingsCommand(planet))
	_, _, err := s.cmd.Run(ctx, "", nil, sshArgs[0], sshArgs[1:]...)
	if err != nil {
		planet.mu.Lock()
		planet.Status = PlanetStatusLost
		planet.Error = err
		planet.mu.Unlock()
		s.bus.Emit(events.Event{
			Kind:     events.KindLemmingsFailed,
			PlanetID: planet.ID,
			Provider: s.provider.Name(),
			Region:   planet.Region,
			Err:      err,
		})
		return fmt.Errorf("planet %d: lemmings run failed: %w", planet.ID, err)
	}

	s.bus.Emit(events.Event{
		Kind:       events.KindLemmingsComplete,
		PlanetID:   planet.ID,
		Provider:   s.provider.Name(),
		Region:     planet.Region,
		PublicIPv4: planet.PublicIPv4,
	})

	fmt.Fprintf(s.out, "  sovereign: planet %d lemmings run complete\n", planet.ID)
	return nil
}

// collectResults collects lemmings report files from each planet
// via SCP concurrently through Commander.
func (s *Sovereign) collectResults() error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, planet := range s.planets {
		planet := planet
		if planet.Status == PlanetStatusLost {
			fmt.Fprintf(s.out, "  sovereign: skipping collection for planet %d -- lost\n",
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
		fmt.Fprintf(s.out, "  sovereign: warning: collection error: %v\n", err)
	}

	return nil
}

// collectFromPlanet SCPs the lemmings report files from a planet
// back to the sovereign working directory via Commander.
func (s *Sovereign) collectFromPlanet(planet *Planet) error {
	planet.mu.Lock()
	planet.Status = PlanetStatusCollecting
	planet.mu.Unlock()

	fmt.Fprintf(s.out, "  sovereign: collecting results from planet %d (%s)...\n",
		planet.ID, planet.PublicIPv4)

	s.bus.Emit(events.Event{
		Kind:       events.KindCollectStart,
		PlanetID:   planet.ID,
		Provider:   s.provider.Name(),
		Region:     planet.Region,
		PublicIPv4: planet.PublicIPv4,
	})

	localDir := s.planetReportDir(planet)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("planet %d: create report dir: %w", planet.ID, err)
	}

	scpArgs := s.scpArgs(planet,
		fmt.Sprintf("%s@%s:~/lemmings/", planet.RemoteUser, planet.PublicIPv4),
		localDir,
		true,
	)
	_, _, err := s.cmd.Run(s.ctx, "", nil, scpArgs[0], scpArgs[1:]...)
	if err != nil {
		planet.mu.Lock()
		planet.Status = PlanetStatusLost
		planet.Error = err
		planet.mu.Unlock()
		s.bus.Emit(events.Event{
			Kind:     events.KindCollectFailed,
			PlanetID: planet.ID,
			Provider: s.provider.Name(),
			Region:   planet.Region,
			Err:      err,
		})
		return fmt.Errorf("planet %d: scp collect failed: %w", planet.ID, err)
	}

	planet.mu.Lock()
	planet.Status = PlanetStatusDone
	planet.CompletedAt = time.Now()
	planet.ReportPath = localDir
	planet.mu.Unlock()

	s.bus.Emit(events.Event{
		Kind:       events.KindCollectComplete,
		PlanetID:   planet.ID,
		Provider:   s.provider.Name(),
		Region:     planet.Region,
		PublicIPv4: planet.PublicIPv4,
	})

	fmt.Fprintf(s.out, "  sovereign: planet %d results collected to %s\n",
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

// copyPlanFile SCPs the local lemmings plan YAML file to the planet
// via Commander.
func (s *Sovereign) copyPlanFile(planet *Planet) error {
	if s.cfg.Plan == "" {
		return nil
	}
	scpArgs := s.scpArgs(planet,
		s.cfg.Plan,
		fmt.Sprintf("%s@%s:/home/%s/lemmings.plan.yml",
			planet.RemoteUser, planet.PublicIPv4, planet.RemoteUser),
		false,
	)
	_, _, err := s.cmd.Run(s.ctx, "", nil, scpArgs[0], scpArgs[1:]...)
	if err != nil {
		return fmt.Errorf("scp plan file to planet %d: %w", planet.ID, err)
	}
	return nil
}

// sshArgs builds the argument slice for an SSH command to a planet.
// The base args include StrictHostKeyChecking=no and ConnectTimeout=30.
// Additional SSH options from s.cfg.SSHOptions are appended before the
// remote command so callers can inject extra flags such as known-hosts
// overrides or identity file options without modifying this function.
func (s *Sovereign) sshArgs(planet *Planet, remoteCmd string) []string {
	args := []string{
		"ssh",
		"-i", s.planetSSHKeyPath(planet),
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=30",
	}
	for _, opt := range s.cfg.SSHOptions {
		args = append(args, "-o", opt)
	}
	args = append(args,
		fmt.Sprintf("%s@%s", planet.RemoteUser, planet.PublicIPv4),
		remoteCmd,
	)
	return args
}

// scpArgs builds the argument slice for an SCP command to or from a planet.
// recursive controls whether -r is passed (needed for directory transfers).
// Additional SSH options from s.cfg.SSHOptions are passed via -o flags.
func (s *Sovereign) scpArgs(planet *Planet, src, dst string, recursive bool) []string {
	args := []string{
		"scp",
		"-i", s.planetSSHKeyPath(planet),
		"-o", "StrictHostKeyChecking=no",
	}
	if recursive {
		args = append(args, "-r")
	}
	for _, opt := range s.cfg.SSHOptions {
		args = append(args, "-o", opt)
	}
	args = append(args, src, dst)
	return args
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
func (s *Sovereign) planetSSHKeyPath(planet *Planet) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "sovereign-planet.pem")
}
