package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
)

// provisionTerrain provisions all planets using the configured provider.
// The first planet is provisioned alone. Remaining planets are
// provisioned concurrently in batches of 3.
// Lost planets are non-fatal -- the run continues with surviving planets.
func (s *Sovereign) provisionTerrain() error {
	if len(s.planets) == 0 {
		return fmt.Errorf("no planets to provision")
	}

	if err := s.resumeOrProvision(s.planets[0]); err != nil {
		return fmt.Errorf("sovereign: first planet failed to provision: %w", err)
	}

	if len(s.planets) == 1 {
		return nil
	}

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
			if err := s.resumeOrProvision(planet); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	for _, err := range errs {
		fmt.Printf("  sovereign: warning: planet provisioning error: %v\n", err)
	}

	return nil
}

// resumeOrProvision checks the ledger and provider state to determine
// whether a planet needs fresh provisioning or can resume from a
// previous interrupted run.
func (s *Sovereign) resumeOrProvision(planet *Planet) error {
	// already completed in a previous run -- skip entirely
	if s.ledger.PlanetCompleted(planet.ID) {
		fmt.Printf("  sovereign: planet %d already completed -- skipping\n", planet.ID)
		planet.mu.Lock()
		planet.Status = PlanetStatusDone
		planet.mu.Unlock()
		return nil
	}

	// already provisioned -- hydrate from provider state and continue
	if s.ledger.PlanetProvisioned(planet.ID) {
		fmt.Printf("  sovereign: planet %d already provisioned -- resuming\n", planet.ID)
		return s.hydrateFromProviderState(planet)
	}

	// check provider state for partial progress
	spec := s.planetSpec(planet)
	state, err := s.provider.LoadState(spec)
	if err != nil {
		return fmt.Errorf("planet %d: load provider state: %w", planet.ID, err)
	}

	if state != nil {
		switch state.Status {
		case tfprovider.LifecycleApplied:
			// instance is running -- hydrate and continue
			return s.hydratePlanetFromState(planet, state)

		case tfprovider.LifecycleApplying, tfprovider.LifecycleInterrupted:
			// interrupted mid-apply -- let InitOrApply sort it out
			fmt.Printf("  sovereign: planet %d interrupted -- resuming apply\n", planet.ID)
			return s.applyAndHydrate(planet)

		case tfprovider.LifecycleInitialized:
			// init succeeded but apply never ran
			fmt.Printf("  sovereign: planet %d initialized -- running apply\n", planet.ID)
			return s.applyAndHydrate(planet)
		}
	}

	// no usable state -- provision from scratch
	return s.provisionPlanet(planet)
}

// provisionPlanet renders HCL, runs terraform init and apply,
// reads the output, and marks the planet running.
func (s *Sovereign) provisionPlanet(planet *Planet) error {
	planet.mu.Lock()
	planet.Status = PlanetStatusProvisioning
	planet.ProvisionedAt = time.Now()
	planet.mu.Unlock()

	fmt.Printf("  sovereign: provisioning planet %d in %s...\n",
		planet.ID, planet.Region)

	s.ledger.Append(LedgerEvent{
		PlanetID:  planet.ID,
		Provider:  s.provider.Name(),
		Region:    planet.Region,
		EventType: LedgerEventProvisionStart,
	})

	// render HCL into planet work dir
	if err := s.provider.RenderHCL(s.hclSpec(planet)); err != nil {
		return s.markLost(planet, LedgerEventProvisionFailed, err)
	}

	spec := s.planetSpec(planet)

	// save initialized state
	if err := s.saveProviderState(planet, tfprovider.LifecycleUninitialized, nil); err != nil {
		return s.markLost(planet, LedgerEventProvisionFailed, err)
	}

	// terraform init
	s.ledger.Append(LedgerEvent{
		PlanetID:  planet.ID,
		EventType: LedgerEventInitStart,
	})
	if err := s.provider.Init(s.ctx, spec); err != nil {
		s.saveProviderState(planet, tfprovider.LifecycleFailed, nil)
		return s.markLost(planet, LedgerEventInitFailed, err)
	}
	s.ledger.Append(LedgerEvent{
		PlanetID:  planet.ID,
		EventType: LedgerEventInitComplete,
	})
	s.saveProviderState(planet, tfprovider.LifecycleInitialized, nil)

	return s.applyAndHydrate(planet)
}

// applyAndHydrate runs terraform apply, reads output, and marks
// the planet running. Called both on fresh provision and on resume.
func (s *Sovereign) applyAndHydrate(planet *Planet) error {
	spec := s.planetSpec(planet)

	s.saveProviderState(planet, tfprovider.LifecycleApplying, nil)
	s.ledger.Append(LedgerEvent{
		PlanetID:  planet.ID,
		EventType: LedgerEventApplyStart,
	})

	if err := s.provider.Apply(s.ctx, spec); err != nil {
		s.saveProviderState(planet, tfprovider.LifecycleInterrupted, nil)
		return s.markLost(planet, LedgerEventApplyFailed, err)
	}

	s.ledger.Append(LedgerEvent{
		PlanetID:  planet.ID,
		EventType: LedgerEventApplyComplete,
	})

	out, err := s.provider.Output(s.ctx, spec)
	if err != nil {
		s.saveProviderState(planet, tfprovider.LifecycleFailed, nil)
		return s.markLost(planet, LedgerEventProvisionFailed, err)
	}

	return s.hydratePlanetFromOutput(planet, out)
}

// hydratePlanetFromOutput populates planet fields from a ProvisionOutput
// and marks the planet running.
func (s *Sovereign) hydratePlanetFromOutput(planet *Planet, out *tfprovider.ProvisionOutput) error {
	planet.mu.Lock()
	planet.PublicIPv4 = out.PublicIPv4
	planet.InstanceID = out.InstanceID
	planet.Status = PlanetStatusRunning
	planet.mu.Unlock()

	s.saveProviderState(planet, tfprovider.LifecycleApplied, out)
	s.ledger.Append(LedgerEvent{
		PlanetID:   planet.ID,
		EventType:  LedgerEventProvisionComplete,
		InstanceID: out.InstanceID,
		PublicIPv4: out.PublicIPv4,
	})

	fmt.Printf("  sovereign: planet %d provisioned -- %s (%s)\n",
		planet.ID, out.PublicIPv4, planet.Region)

	return nil
}

// hydratePlanetFromState populates planet fields from an existing
// ProviderState. Used when resuming a run where the planet was
// already successfully provisioned.
func (s *Sovereign) hydratePlanetFromState(planet *Planet, state *tfprovider.ProviderState) error {
	planet.mu.Lock()
	planet.PublicIPv4 = state.PublicIPv4
	planet.InstanceID = state.InstanceID
	planet.Status = PlanetStatusRunning
	planet.mu.Unlock()

	fmt.Printf("  sovereign: planet %d resumed -- %s (%s)\n",
		planet.ID, state.PublicIPv4, planet.Region)

	return nil
}

// hydrateFromProviderState loads the provider state file and
// hydrates the planet from it. Used when the ledger indicates
// a planet was provisioned but sovereign needs the IP and instance ID.
func (s *Sovereign) hydrateFromProviderState(planet *Planet) error {
	spec := s.planetSpec(planet)
	state, err := s.provider.LoadState(spec)
	if err != nil {
		return fmt.Errorf("planet %d: load provider state for resume: %w", planet.ID, err)
	}
	if state == nil {
		return fmt.Errorf("planet %d: ledger says provisioned but no provider state found",
			planet.ID)
	}
	return s.hydratePlanetFromState(planet, state)
}

// destroyTerrain destroys all provisioned planets concurrently.
// All planets are attempted regardless of individual failures.
// Returns an error if any planet fails to terminate.
func (s *Sovereign) destroyTerrain() error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for _, planet := range s.planets {
		planet := planet
		if planet.Status == PlanetStatusLost && planet.InstanceID == "" {
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
			fmt.Fprintf(os.Stderr, "  sovereign: destroy error: %v\n", err)
		}
		return fmt.Errorf("sovereign: %d planet(s) failed to terminate -- manual intervention required",
			len(errs))
	}
	return nil
}

// destroyPlanet runs terraform destroy for a single planet.
func (s *Sovereign) destroyPlanet(planet *Planet) error {
	fmt.Printf("  sovereign: destroying planet %d (%s)...\n",
		planet.ID, planet.InstanceID)

	s.ledger.Append(LedgerEvent{
		PlanetID:   planet.ID,
		EventType:  LedgerEventDestroyStart,
		InstanceID: planet.InstanceID,
	})

	s.saveProviderState(planet, tfprovider.LifecycleDestroying, nil)

	spec := s.planetSpec(planet)
	if err := s.provider.Destroy(s.ctx, spec); err != nil {
		s.ledger.Append(LedgerEvent{
			PlanetID:  planet.ID,
			EventType: LedgerEventDestroyFailed,
			Error:     err.Error(),
		})
		return fmt.Errorf("planet %d (%s): destroy failed: %w",
			planet.ID, planet.InstanceID, err)
	}

	s.saveProviderState(planet, tfprovider.LifecycleDestroyed, nil)
	s.ledger.Append(LedgerEvent{
		PlanetID:   planet.ID,
		EventType:  LedgerEventDestroyComplete,
		InstanceID: planet.InstanceID,
	})

	fmt.Printf("  sovereign: planet %d destroyed\n", planet.ID)
	return nil
}

// -- private helpers ---------------------------------------------------------

// markLost sets the planet status to lost, appends a failure event
// to the ledger, and returns the original error wrapped with planet context.
func (s *Sovereign) markLost(planet *Planet, event LedgerEventType, err error) error {
	planet.mu.Lock()
	planet.Status = PlanetStatusLost
	planet.Error = err
	planet.mu.Unlock()

	s.ledger.Append(LedgerEvent{
		PlanetID:  planet.ID,
		EventType: event,
		Error:     err.Error(),
	})

	return fmt.Errorf("planet %d: %w", planet.ID, err)
}

// saveProviderState writes the current provider state to disk.
// Errors are logged as warnings -- a state save failure does not
// abort provisioning since the operation itself may have succeeded.
func (s *Sovereign) saveProviderState(planet *Planet, status tfprovider.LifecycleStatus, out *tfprovider.ProvisionOutput) {
	state := &tfprovider.ProviderState{
		PlanetID:     planet.ID,
		Provider:     s.provider.Name(),
		Region:       planet.Region,
		Status:       status,
		TerraformDir: filepath.Join(s.planetWorkDir(planet), "terraform"),
		StateFile:    filepath.Join(s.planetWorkDir(planet), "terraform", "terraform.tfstate"),
	}

	if planet.InstanceID != "" {
		state.InstanceID = planet.InstanceID
	}
	if planet.PublicIPv4 != "" {
		state.PublicIPv4 = planet.PublicIPv4
	}

	if out != nil {
		state.InstanceID = out.InstanceID
		state.PublicIPv4 = out.PublicIPv4
		state.PrivateIPv4 = out.PrivateIPv4
	}

	switch status {
	case tfprovider.LifecycleInitialized:
		state.InitializedAt = time.Now().UTC()
	case tfprovider.LifecycleApplied:
		state.AppliedAt = time.Now().UTC()
	case tfprovider.LifecycleDestroyed:
		state.DestroyedAt = time.Now().UTC()
	case tfprovider.LifecycleInterrupted:
		state.InterruptedAt = time.Now().UTC()
	}

	spec := s.planetSpec(planet)
	if err := s.provider.SaveState(spec, state); err != nil {
		fmt.Printf("  sovereign: warning: failed to save provider state for planet %d: %v\n",
			planet.ID, err)
	}
}

// planetSpec constructs a PlanetSpec from a planet for provider calls.
func (s *Sovereign) planetSpec(planet *Planet) tfprovider.PlanetSpec {
	return tfprovider.PlanetSpec{
		PlanetID: planet.ID,
		WorkDir:  s.planetWorkDir(planet),
		Region:   planet.Region,
		Zone:     planet.Region + "a",
	}
}

// hclSpec constructs an HCLSpec from a planet and sovereign config
// for provider HCL rendering.
func (s *Sovereign) hclSpec(planet *Planet) tfprovider.HCLSpec {
	home, _ := os.UserHomeDir()
	return tfprovider.HCLSpec{
		PlanetID:        planet.ID,
		Slug:            fmt.Sprintf("sovereign-planet-%d", planet.ID),
		Region:          planet.Region,
		Zone:            planet.Region + "a",
		InstanceType:    planet.InstanceType,
		RemoteUser:      s.cfg.RemoteUser,
		AMIOS:           s.cfg.AMIOS,
		AMIOwners:       s.cfg.AMIOwners,
		SubnetID:        os.Getenv("SOVEREIGN_SUBNET_ID"),
		SecurityGroupID: os.Getenv("SOVEREIGN_SECURITY_GROUP"),
		RootVolumeSize:  s.cfg.RootVolumeSize,
		RootVolumeType:  s.cfg.RootVolumeType,
		TempVolumeSize:  s.cfg.TempVolumeSize,
		TempVolumeType:  s.cfg.TempVolumeType,
		SSHKeyPath:      filepath.Join(home, ".ssh", "sovereign-planet.pem"),
		GoVersion:       s.cfg.GoVersion,
		LemmingsPackage: s.cfg.LemmingsPackage,
		Hit:             s.cfg.Hit,
		Terrain:         s.cfg.Terrain,
		Pack:            s.cfg.Pack,
		Until:           s.cfg.Until.String(),
		Ramp:            s.cfg.Ramp.String(),
		SaveTo:          fmt.Sprintf("/home/%s/lemmings", s.cfg.RemoteUser),
		WorkDir:         s.planetWorkDir(planet),
		StateFilePath: filepath.Join(
			s.planetWorkDir(planet), "terraform", "terraform.tfstate"),
	}
}

// planetWorkDir returns the working directory for a planet.
func (s *Sovereign) planetWorkDir(planet *Planet) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".sovereignd", "planets",
		fmt.Sprintf("planet-%d", planet.ID))
}
