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

// provisionTerrain renders a topobuilder YAML config for each planet
// and calls topobuilder to provision the AWS infrastructure.
//
// The first planet is provisioned alone. Remaining planets are
// provisioned in concurrent batches of 3 following the same
// semaphore pattern topobuilder uses internally for zones.
//
// A planet that fails to provision within cfg.ProvisionTimeout
// is marked PlanetStatusLost. Lost planets are skipped during
// lemmings deployment and collection but are noted in the final
// report. The run continues with remaining planets.
func (s *Sovereign) provisionTerrain() error {
    if len(s.planets) == 0 {
        return fmt.Errorf("no planets to provision")
    }

    // Provision first planet alone -- topobuilder requires the
    // first zone to complete before broadcasting to remaining zones.
    // Sovereign mirrors this pattern at the planet level.
    if err := s.provisionPlanet(s.planets[0]); err != nil {
        return fmt.Errorf("sovereign: first planet failed to provision: %w", err)
    }

    if len(s.planets) == 1 {
        return nil
    }

    // Provision remaining planets in batches of 3
    remaining := s.planets[1:]
    sem := make(chan struct{}, 3)
    var wg sync.WaitGroup
    var mu sync.Mutex
    var errs []error

    for _, planet := range remaining {
        planet := planet
        wg.Add(1)
        sem <- struct{}{}
        go func() {
            defer wg.Done()
            defer func() { <-sem }()
            if err := s.provisionPlanet(planet); err != nil {
                mu.Lock()
                errs = append(errs, err)
                mu.Unlock()
            }
        }()
    }
    wg.Wait()

    // Lost planets are non-fatal -- report them but continue
    for _, err := range errs {
        fmt.Printf("  sovereign: warning: planet provisioning error: %v\n", err)
    }

    return nil
}

// provisionPlanet renders the topobuilder YAML for a single planet,
// writes it to a temp directory, and invokes the topobuilder binary.
//
// On success the planet's PublicIPv4 and InstanceID are populated
// from topobuilder's state output and the planet status is set to
// PlanetStatusRunning.
//
// On failure or timeout the planet status is set to PlanetStatusLost.
func (s *Sovereign) provisionPlanet(planet *Planet) error {
    planet.mu.Lock()
    planet.Status = PlanetStatusProvisioning
    planet.ProvisionedAt = time.Now()
    planet.mu.Unlock()

    fmt.Printf("  sovereign: provisioning planet %d in %s...\n",
        planet.ID, planet.Region)

    // Render the topobuilder YAML for this planet
    configPath, err := s.renderTopobuilderConfig(planet)
    if err != nil {
        planet.mu.Lock()
        planet.Status = PlanetStatusLost
        planet.Error = err
        planet.mu.Unlock()
        return fmt.Errorf("planet %d: render topobuilder config: %w", planet.ID, err)
    }

    // Invoke topobuilder with a provision timeout
    ctx, cancel := context.WithTimeout(s.ctx, s.cfg.ProvisionTimeout)
    defer cancel()

    stateFile := filepath.Join(
        filepath.Dir(configPath),
        fmt.Sprintf("planet-%d.tbstate", planet.ID),
    )

    cmd := exec.CommandContext(ctx, "topobuilder",
        "-config", configPath,
        "-state", stateFile,
        "-mode", "production",
    )
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        planet.mu.Lock()
        planet.Status = PlanetStatusLost
        planet.Error = err
        planet.mu.Unlock()
        return fmt.Errorf("planet %d: topobuilder provision failed: %w", planet.ID, err)
    }

    // Read planet IP and instance ID from topobuilder state file
    if err := s.hydratePlanetFromState(planet, stateFile); err != nil {
        planet.mu.Lock()
        planet.Status = PlanetStatusLost
        planet.Error = err
        planet.mu.Unlock()
        return fmt.Errorf("planet %d: read topobuilder state: %w", planet.ID, err)
    }

    planet.mu.Lock()
    planet.Status = PlanetStatusRunning
    planet.mu.Unlock()

    fmt.Printf("  sovereign: planet %d provisioned -- %s (%s)\n",
        planet.ID, planet.PublicIPv4, planet.Region)

    return nil
}

// destroyTerrain calls topobuilder destroy for each planet concurrently.
// All planets are destroyed regardless of individual failures.
//
// A planet that fails to terminate within cfg.TerminateTimeout is
// logged with its InstanceID for manual intervention. Sovereign
// exits with a non-zero status if any planet fails to terminate.
func (s *Sovereign) destroyTerrain() error {
    var wg sync.WaitGroup
    var mu sync.Mutex
    var errs []error

    for _, planet := range s.planets {
        planet := planet
        if planet.Status == PlanetStatusLost &&
            planet.InstanceID == "" {
            // Planet never provisioned -- nothing to destroy
            continue
        }
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := s.destroyPlanet(planet); err != nil {
                mu.Lock()
                errs = append(errs, err)
                mu.Unlock()
            }
        }()
    }
    wg.Wait()

    if len(errs) > 0 {
        for _, err := range errs {
            fmt.Fprintf(os.Stderr,
                "  sovereign: destroy error: %v\n", err)
        }
        return fmt.Errorf("sovereign: %d planet(s) failed to terminate -- manual intervention required",
            len(errs))
    }
    return nil
}

// destroyPlanet invokes topobuilder destroy for a single planet.
// If termination does not complete within cfg.TerminateTimeout
// the planet's InstanceID is logged for manual intervention.
func (s *Sovereign) destroyPlanet(planet *Planet) error {
    fmt.Printf("  sovereign: destroying planet %d (%s)...\n",
        planet.ID, planet.InstanceID)

    configPath := s.planetConfigPath(planet)
    stateFile := filepath.Join(
        filepath.Dir(configPath),
        fmt.Sprintf("planet-%d.tbstate", planet.ID),
    )

    ctx, cancel := context.WithTimeout(s.ctx, s.cfg.TerminateTimeout)
    defer cancel()

    cmd := exec.CommandContext(ctx, "topobuilder",
        "-config", configPath,
        "-state", stateFile,
        "-mode", "production",
        "-destroy",
    )
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    if err := cmd.Run(); err != nil {
        return fmt.Errorf("planet %d (%s): topobuilder destroy failed: %w",
            planet.ID, planet.InstanceID, err)
    }

    fmt.Printf("  sovereign: planet %d destroyed\n", planet.ID)
    return nil
}

// renderTopobuilderConfig generates a topobuilder-compatible YAML
// config file for a single planet and writes it to the sovereign
// working directory. Returns the path to the written config file.
func (s *Sovereign) renderTopobuilderConfig(planet *Planet) (string, error) {
    dir := s.planetWorkDir(planet)
    if err := os.MkdirAll(dir, 0700); err != nil {
        return "", fmt.Errorf("create planet work dir: %w", err)
    }

    configPath := filepath.Join(dir, "topobuilder.yml")

    // TODO: render full topobuilder YAML from SovereignConfig and
    // planet fields. The rendered config instructs topobuilder to:
    //   -- provision one AWS instance in planet.Region
    //   -- install Go via igo
    //   -- install lemmings via go install
    //   -- copy the lemmings plan file if cfg.Plan is set
    //   -- run lemmings with the sovereign-generated flags
    //   -- expose the StatusAgent on the configured port
    //
    // The topobuilder YAML template lives in templates/topobuilder.yml.go
    // and is rendered via text/template with planet and config values.

    _ = configPath
    return configPath, fmt.Errorf("renderTopobuilderConfig: not yet implemented")
}

// hydratePlanetFromState reads the topobuilder state file after a
// successful provision and populates the planet's PublicIPv4 and
// InstanceID fields.
func (s *Sovereign) hydratePlanetFromState(planet *Planet, stateFile string) error {
    // TODO: read and unmarshal the topobuilder TBState YAML file
    // and extract the first instance's PublicIPv4 and InstanceID
    // into the planet struct.
    //
    // The TBState structure from topobuilder is:
    //   state.Cluster.Zones[0].Instances[0].PublicIPv4
    //   state.Cluster.Zones[0].Instances[0].ID
    _ = stateFile
    return fmt.Errorf("hydratePlanetFromState: not yet implemented")
}

// planetWorkDir returns the working directory for a planet's
// topobuilder config, state file, and collected reports.
func (s *Sovereign) planetWorkDir(planet *Planet) string {
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".sovereignd", "planets",
        fmt.Sprintf("planet-%d", planet.ID))
}

// planetConfigPath returns the expected topobuilder config path
// for a planet without creating it.
func (s *Sovereign) planetConfigPath(planet *Planet) string {
    return filepath.Join(s.planetWorkDir(planet), "topobuilder.yml")
}​​​​​​​​​​​​​​​​
