package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// saveState marshals the current SovereignState to YAML and writes
// it to the state file. The state directory is created with mode
// 0700 if it does not already exist.
//
// State is saved after terrain provisioning so that a crash during
// lemmings execution or result collection does not orphan AWS
// instances. On restart sovereign reads the state file, finds the
// existing planet InstanceIDs, and resumes from collection rather
// than provisioning new instances.
func (s *Sovereign) saveState() error {
	dir := filepath.Dir(s.stateFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("state: mkdir %s: %w", dir, err)
	}

	s.mu.Lock()
	s.state.Planets = s.planets
	s.mu.Unlock()

	payload, err := yaml.Marshal(s.state)
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}

	file, err := os.OpenFile(s.stateFile,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("state: open %s: %w", s.stateFile, err)
	}
	defer func() {
		_ = file.Sync()
		_ = file.Close()
	}()

	n, err := file.Write(payload)
	if err != nil {
		return fmt.Errorf("state: write %s: %w", s.stateFile, err)
	}
	if n != len(payload) {
		return fmt.Errorf("state: incomplete write to %s: wrote %d of %d bytes",
			s.stateFile, n, len(payload))
	}

	return nil
}

// loadState reads and unmarshals the sovereign state file from disk.
// Returns nil, nil if no state file exists -- not an error.
// Returns an error if the file exists but cannot be read or parsed.
func (s *Sovereign) loadState() (*SovereignState, error) {
	if _, err := os.Stat(s.stateFile); os.IsNotExist(err) {
		return nil, nil
	}

	contents, err := os.ReadFile(s.stateFile)
	if err != nil {
		return nil, fmt.Errorf("state: read %s: %w", s.stateFile, err)
	}

	var state SovereignState
	if err := yaml.Unmarshal(contents, &state); err != nil {
		return nil, fmt.Errorf("state: parse %s: %w", s.stateFile, err)
	}

	return &state, nil
}

// cleanState removes the state file after a successful sovereign run.
// A missing state file is not an error -- cleanState is safe to call
// even if saveState was never called.
func (s *Sovereign) cleanState() {
	if err := os.Remove(s.stateFile); err != nil && !os.IsNotExist(err) {
		fmt.Printf("  sovereign: warning: failed to remove state file %s: %v\n",
			s.stateFile, err)
	}
}

// resolveStateFile determines the state file path preferring
// .sovereignd in the current working directory over ~/.sovereignd.
//
// The local path is preferred so that multiple sovereign invocations
// in different project directories maintain isolated state without
// interfering with each other.
//
// Path resolution order:
//
//  1. ./.sovereignd/state.yml  (project-local, preferred)
//  2. ~/.sovereignd/state.yml  (user-global, fallback)
func (s *Sovereign) resolveStateFile() (string, error) {
	localDir := ".sovereignd"
	if _, err := os.Stat(localDir); err == nil {
		return filepath.Join(localDir, "state.yml"), nil
	}

	// Local directory does not exist yet -- check if a local state
	// file would be appropriate by testing write access to cwd
	cwd, err := os.Getwd()
	if err == nil {
		testPath := filepath.Join(cwd, localDir)
		if mkErr := os.MkdirAll(testPath, 0700); mkErr == nil {
			return filepath.Join(testPath, "state.yml"), nil
		}
	}

	// Fall back to home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("state: resolve home dir: %w", err)
	}
	return filepath.Join(home, ".sovereignd", "state.yml"), nil
}

// planetState returns the current state snapshot of a single planet
// safe for serialization. The planet mutex is held during the read
// to prevent a concurrent status update from producing inconsistent
// state on disk.
func planetState(planet *Planet) Planet {
	planet.mu.Lock()
	defer planet.mu.Unlock()
	return Planet{
		ID:            planet.ID,
		Region:        planet.Region,
		InstanceType:  planet.InstanceType,
		PublicIPv4:    planet.PublicIPv4,
		InstanceID:    planet.InstanceID,
		Status:        planet.Status,
		ProvisionedAt: planet.ProvisionedAt,
		CompletedAt:   planet.CompletedAt,
		ReportPath:    planet.ReportPath,
	}
}

// lostPlanets returns all planets currently in PlanetStatusLost.
// Used by the final summary and the report to enumerate instances
// that may require manual intervention in AWS.
func (s *Sovereign) lostPlanets() []*Planet {
	var lost []*Planet
	for _, p := range s.planets {
		if p.Status == PlanetStatusLost {
			lost = append(lost, p)
		}
	}
	return lost
}

// pendingPlanets returns all planets that have not yet reached
// a terminal status -- PlanetStatusDone or PlanetStatusLost.
// Used to determine whether a resumed run has work remaining.
func (s *Sovereign) pendingPlanets() []*Planet {
	var pending []*Planet
	for _, p := range s.planets {
		if p.Status != PlanetStatusDone && p.Status != PlanetStatusLost {
			pending = append(pending, p)
		}
	}
	return pending
}

// allPlanetsTerminal returns true when every planet has reached
// either PlanetStatusDone or PlanetStatusLost. Used to determine
// when sovereign can proceed to terrain destruction.
func (s *Sovereign) allPlanetsTerminal() bool {
	for _, p := range s.planets {
		if p.Status != PlanetStatusDone && p.Status != PlanetStatusLost {
			return false
		}
	}
	return true
}
