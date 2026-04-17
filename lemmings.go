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
//
// The run deadline is cfg.Ramp + cfg.Until + 5 minutes overhead.
// If lemmings does not complete within the deadline the planet is
// marked PlanetStatusLost and the SSH session is terminated.
func (s *Sovereign) runLemmingsOnPlanet(planet *Planet) error {
    fmt.Printf("  sovereign: starting lemmings on planet %d (%s)...\n",
        planet.ID, planet.PublicIPv4)

    // Copy lemmings config to the planet
    if err := s.copyLemmingsConfig(planet); err != nil {
        planet.mu.Lock()
        planet.Status = PlanetStatusLost
        planet.Error = err
        planet.mu.Unlock()
        return fmt.Errorf("planet %d: copy lemmings config: %w", planet.ID, err)
    }

    // Copy plan file if configured
    if s.cfg.Plan != "" {
        if err := s.copyPlanFile(planet); err != nil {
            planet.mu.Lock()
            planet.Status = PlanetStatusLost
            planet.Error = err
            planet.mu.Unlock()
            return fmt.Errorf("planet %d: copy plan file: %w", planet.ID, err)
        }
    }

    // Build the run deadline -- ramp + until + 5 minute overhead
    deadline := s.cfg.Ramp + s.cfg.Until + 5*time.Minute
    ctx, cancel := context.WithTimeout(s.ctx, deadline)
    defer cancel()

    // Build the lemmings command to run on the remote instance
    lemmingsCmd := s.buildLemmingsCommand(planet)

    // Execute lemmings on the remote instance via SSH
    sshKeyPath := s.planetSSHKeyPath(planet)
    cmd := exec.CommandContext(ctx,
        "ssh",
        "-i", sshKeyPath,
        "-o", "StrictHostKeyChecking=no",
        "-o", "ConnectTimeout=30",
        fmt.Sprintf("ubuntu@%s", planet.PublicIPv4),
        lemmingsCmd,
    )
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        planet.mu.Lock()
        planet.Status = PlanetStatusLost
        planet.Error = err
        planet.mu.Unlock()
        return fmt.Errorf("planet %d: lemmings run failed: %w", planet.ID, err)
    }

    fmt.Printf("  sovereign: planet %d lemmings run complete\n", planet.ID)
    return nil
}

// collectResults collects LifeLog and report files from each planet
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
// back to the sovereign working directory. On success the planet's
// ReportPath is populated and its status is set to PlanetStatusDone.
func (s *Sovereign) collectFromPlanet(planet *Planet) error {
    planet.mu.Lock()
    planet.Status = PlanetStatusCollecting
    planet.mu.Unlock()

    fmt.Printf("  sovereign: collecting results from planet %d (%s)...\n",
        planet.ID, planet.PublicIPv4)

    localDir := s.planetReportDir(planet)
    if err := os.MkdirAll(localDir, 0755); err != nil {
        return fmt.Errorf("planet %d: create report dir: %w", planet.ID, err)
    }

    sshKeyPath := s.planetSSHKeyPath(planet)

    // SCP the lemmings report directory from the remote instance
    // lemmings writes reports to ~/lemmings/<domain>/ by default
    cmd := exec.CommandContext(s.ctx,
        "scp",
        "-i", sshKeyPath,
        "-o", "StrictHostKeyChecking=no",
        "-r",
        fmt.Sprintf("ubuntu@%s:~/lemmings/", planet.PublicIPv4),
        localDir,
    )
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        planet.mu.Lock()
        planet.Status = PlanetStatusLost
        planet.Error = err
        planet.mu.Unlock()
        return fmt.Errorf("planet %d: scp collect failed: %w", planet.ID, err)
    }

    planet.mu.Lock()
    planet.Status = PlanetStatusDone
    planet.CompletedAt = time.Now()
    planet.ReportPath = localDir
    planet.mu.Unlock()

    fmt.Printf("  sovereign: planet %d results collected to %s\n",
        planet.ID, localDir)

    return nil
}

// aggregateReport merges the LifeLog and report files collected from
// all completed planets into a single unified report delivered to
// the configured save-to destinations.
func (s *Sovereign) aggregateReport() error {
    completed := s.completedPlanets()
    if len(completed) == 0 {
        return fmt.Errorf("no planets completed successfully -- no report to aggregate")
    }

    fmt.Printf("  sovereign: aggregating reports from %d planet(s)...\n",
        len(completed))

    // TODO: read each planet's lemmings report files from ReportPath
    // merge LifeLogs into a unified ledger
    // compute global percentiles across all planets
    // add per-planet breakdown section to the unified report
    // deliver unified report to each save-to destination

    return fmt.Errorf("aggregateReport: not yet implemented")
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
        cmd += fmt.Sprintf(" -plan ~/lemmings.plan.yml")
    }
    return cmd
}

// copyLemmingsConfig writes a minimal lemmings environment config
// to the planet via SCP for use during the remote run.
// Currently lemmings is fully flag-driven so no config file is
// required beyond the plan file. This function is a hook for
// future lemmings config file support.
func (s *Sovereign) copyLemmingsConfig(planet *Planet) error {
    // lemmings v0.0.1 is fully flag-driven.
    // No config file copy is required at this time.
    // This function exists as an extension point for future
    // lemmings config file support.
    return nil
}

// copyPlanFile SCPs the local lemmings plan YAML file to the planet.
func (s *Sovereign) copyPlanFile(planet *Planet) error {
    if s.cfg.Plan == "" {
        return nil
    }

    sshKeyPath := s.planetSSHKeyPath(planet)

    cmd := exec.CommandContext(s.ctx,
        "scp",
        "-i", sshKeyPath,
        "-o", "StrictHostKeyChecking=no",
        s.cfg.Plan,
        fmt.Sprintf("ubuntu@%s:~/lemmings.plan.yml", planet.PublicIPv4),
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
// The key is written by topobuilder during provisioning and lives
// in the planet's working directory.
func (s *Sovereign) planetSSHKeyPath(planet *Planet) string {
    return filepath.Join(s.planetWorkDir(planet), "planet.pem")
}​​​​​​​​​​​​​​​​
