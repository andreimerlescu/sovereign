package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"
)

// SovereignConfig is the single source of truth flowing from CLI
// or config file into every layer of sovereign.
type SovereignConfig struct {
    Hit              string
    Planets          int
    Terrain          int64
    Pack             int64
    Until            time.Duration
    Ramp             time.Duration
    Provider         string
    Plan             string
    Regions          []string
    InstanceType     string
    SaveTo           []string
    ProvisionTimeout time.Duration
    TerminateTimeout time.Duration
    Yes              bool
    DryRun           bool
    ConfigFile       string
    Version          string
}

// PlanetStatus represents the current state of a single planet.
type PlanetStatus string

const (
    PlanetStatusPending      PlanetStatus = "pending"
    PlanetStatusProvisioning PlanetStatus = "provisioning"
    PlanetStatusRunning      PlanetStatus = "running"
    PlanetStatusCollecting   PlanetStatus = "collecting"
    PlanetStatusDone         PlanetStatus = "done"
    PlanetStatusLost         PlanetStatus = "lost"
)

// Planet represents a single remote cloud instance running lemmings.
type Planet struct {
    ID               int
    Region           string
    Provider         string
    InstanceType     string
    PublicIPv4       string
    Status           PlanetStatus
    ProvisionedAt    time.Time
    CompletedAt      time.Time
    ReportPath       string
    Error            error
    mu               sync.Mutex
}

// SovereignState is persisted to disk so sovereign can resume
// after a crash without orphaning cloud infrastructure.
type SovereignState struct {
    Version     string        `yaml:"Version"`
    StartedAt   time.Time     `yaml:"StartedAt"`
    Config      SovereignConfig `yaml:"Config"`
    Planets     []*Planet     `yaml:"Planets"`
    StateFile   string        `yaml:"-"`
}

// Sovereign is the top level coordinator. One sovereign per invocation.
type Sovereign struct {
    cfg       SovereignConfig
    ctx       context.Context
    cancel    context.CancelFunc
    planets   []*Planet
    state     *SovereignState
    stateFile string
    startedAt time.Time
    mu        sync.Mutex
}

// NewSovereign constructs a Sovereign, loads any existing state from
// disk, validates the configuration, and prepares for Run.
func NewSovereign(ctx context.Context, cfg SovereignConfig) (*Sovereign, error) {
    ctx, cancel := context.WithCancel(ctx)

    s := &Sovereign{
        cfg:       cfg,
        ctx:       ctx,
        cancel:    cancel,
        startedAt: time.Now(),
    }

    // Determine state file location
    stateFile, err := s.resolveStateFile()
    if err != nil {
        cancel()
        return nil, fmt.Errorf("sovereign: resolve state file: %w", err)
    }
    s.stateFile = stateFile

    // Load existing state if present -- allows resume after crash
    existing, loadErr := s.loadState()
    if loadErr == nil && existing != nil {
        fmt.Printf("  sovereign: resuming from existing state at %s\n", stateFile)
        s.state = existing
        s.planets = existing.Planets
    } else {
        // Fresh run -- initialize planets from config
        s.planets = s.initPlanets()
        s.state = &SovereignState{
            Version:   cfg.Version,
            StartedAt: s.startedAt,
            Config:    cfg,
            Planets:   s.planets,
            StateFile: stateFile,
        }
    }

    // Validate engine compatibility with remote terrain
    if err := s.validateRemoteConstraint(); err != nil {
        cancel()
        return nil, err
    }

    return s, nil
}

// Run executes the full sovereign lifecycle:
// estimate cost, confirm, provision terrain, run lemmings,
// collect results, aggregate report, destroy terrain.
func (s *Sovereign) Run() error {
    defer s.cancel()

    // Step 1 -- estimate cost
    estimate, err := s.estimateCost()
    if err != nil {
        return fmt.Errorf("sovereign: cost estimation failed: %w", err)
    }
    s.printCostEstimate(estimate)

    // Step 2 -- dry run exits here
    if s.cfg.DryRun {
        fmt.Println("  sovereign: dry-run complete -- no infrastructure provisioned")
        return nil
    }

    // Step 3 -- confirm unless --yes flag set
    if !s.cfg.Yes {
        if err := s.confirmWithUser(); err != nil {
            return fmt.Errorf("sovereign: aborted by user")
        }
    }

    // Step 4 -- provision terrain via topobuilder
    fmt.Println("  sovereign: provisioning terrain...")
    if err := s.provisionTerrain(); err != nil {
        return fmt.Errorf("sovereign: provision terrain: %w", err)
    }

    // Step 5 -- save state so crash recovery is possible
    if err := s.saveState(); err != nil {
        fmt.Printf("  sovereign: warning: failed to save state: %v\n", err)
    }

    // Step 6 -- run lemmings on each planet concurrently
    fmt.Println("  sovereign: deploying lemmings to planets...")
    if err := s.runLemmings(); err != nil {
        return fmt.Errorf("sovereign: run lemmings: %w", err)
    }

    // Step 7 -- collect LifeLogs and reports from each planet
    fmt.Println("  sovereign: collecting results from planets...")
    if err := s.collectResults(); err != nil {
        return fmt.Errorf("sovereign: collect results: %w", err)
    }

    // Step 8 -- aggregate all LifeLogs into unified report
    fmt.Println("  sovereign: aggregating unified report...")
    if err := s.aggregateReport(); err != nil {
        return fmt.Errorf("sovereign: aggregate report: %w", err)
    }

    // Step 9 -- destroy terrain
    fmt.Println("  sovereign: destroying terrain...")
    if err := s.destroyTerrain(); err != nil {
        return fmt.Errorf("sovereign: destroy terrain: %w", err)
    }

    // Step 10 -- clean up state file
    s.cleanState()

    s.printFinalSummary()

    return nil
}

// initPlanets builds the initial planet list from the config.
// Regions are distributed round-robin across the requested planet count.
func (s *Sovereign) initPlanets() []*Planet {
    planets := make([]*Planet, s.cfg.Planets)
    regions := s.cfg.Regions
    if len(regions) == 0 {
        regions = defaultRegions(s.cfg.Provider)
    }
    for i := 0; i < s.cfg.Planets; i++ {
        planets[i] = &Planet{
            ID:           i + 1,
            Region:       regions[i%len(regions)],
            Provider:     s.cfg.Provider,
            InstanceType: s.cfg.InstanceType,
            Status:       PlanetStatusPending,
        }
    }
    return planets
}

// validateRemoteConstraint ensures the selected engine supports
// remote terrain. Only the web engine is permitted on planets.
func (s *Sovereign) validateRemoteConstraint() error {
    // Currently sovereign only supports the web engine.
    // When pkg/lemming engine selection is wired in, this check
    // will inspect the selected engine's IsRemote() method.
    // For now, SQL, NoSQL, and CDN engines are not yet selectable
    // via sovereign -- that constraint is enforced by absence.
    return nil
}

// resolveStateFile determines the state file path, preferring
// .sovereignd in the current working directory over ~/.sovereignd.
func (s *Sovereign) resolveStateFile() (string, error) {
    local := filepath.Join(".sovereignd", "state.yml")
    if _, err := os.Stat(filepath.Dir(local)); err == nil {
        return local, nil
    }
    home, err := os.UserHomeDir()
    if err != nil {
        return "", fmt.Errorf("resolve home dir: %w", err)
    }
    return filepath.Join(home, ".sovereignd", "state.yml"), nil
}

// loadState reads an existing state file from disk.
// Returns nil, nil if no state file exists -- not an error.
func (s *Sovereign) loadState() (*SovereignState, error) {
    if _, err := os.Stat(s.stateFile); os.IsNotExist(err) {
        return nil, nil
    }
    // TODO: unmarshal YAML state file
    return nil, nil
}

// saveState writes the current state to disk for crash recovery.
func (s *Sovereign) saveState() error {
    dir := filepath.Dir(s.stateFile)
    if err := os.MkdirAll(dir, 0700); err != nil {
        return fmt.Errorf("save state mkdir: %w", err)
    }
    // TODO: marshal and write YAML state file
    return nil
}

// cleanState removes the state file after a successful run.
func (s *Sovereign) cleanState() {
    _ = os.Remove(s.stateFile)
}

// confirmWithUser prompts the engineer to confirm before provisioning.
func (s *Sovereign) confirmWithUser() error {
    fmt.Print("  sovereign: proceed with provisioning? [y/N] ")
    var response string
    _, err := fmt.Scanln(&response)
    if err != nil || (response != "y" && response != "Y") {
        return fmt.Errorf("aborted")
    }
    return nil
}

// printCostEstimate prints the projected cost before provisioning.
func (s *Sovereign) printCostEstimate(estimate *CostEstimate) {
    fmt.Println()
    fmt.Println("  sovereign: projected cost estimate")
    fmt.Println("  ─────────────────────────────────")
    for _, planet := range estimate.Planets {
        fmt.Printf("    planet %d  %s  %s  $%.4f/hr\n",
            planet.ID, planet.Region, planet.InstanceType, planet.HourlyRate)
    }
    fmt.Printf("\n    total planets:   %d\n", len(estimate.Planets))
    fmt.Printf("    est. duration:   %s\n", estimate.Duration.Round(time.Second))
    fmt.Printf("    projected cost:  $%.4f\n", estimate.TotalCost)
    fmt.Println()
}

// printFinalSummary writes the post-run block to STDOUT.
func (s *Sovereign) printFinalSummary() {
    elapsed := time.Since(s.startedAt).Round(time.Second)
    lost := 0
    done := 0
    for _, p := range s.planets {
        switch p.Status {
        case PlanetStatusDone:
            done++
        case PlanetStatusLost:
            lost++
        }
    }
    fmt.Println()
    fmt.Println("─────────────────────────────────────────")
    fmt.Printf("  sovereign v%s -- final summary\n", s.cfg.Version)
    fmt.Println("─────────────────────────────────────────")
    fmt.Printf("  target:    %s\n", s.cfg.Hit)
    fmt.Printf("  planets:   %d provisioned\n", len(s.planets))
    fmt.Printf("  completed: %d planets\n", done)
    if lost > 0 {
        fmt.Printf("  lost:      %d planets (see report for details)\n", lost)
    }
    fmt.Printf("  elapsed:   %s\n", elapsed)
    fmt.Println("─────────────────────────────────────────")
}

// defaultRegions returns a sensible default region list for a provider
// when no regions are specified in the config.
func defaultRegions(provider string) []string {
    switch provider {
    case "digitalocean":
        return []string{"nyc3", "sfo3", "ams3", "sgp1", "lon1"}
    case "aws":
        return []string{"us-east-1", "us-west-1", "eu-west-1", "ap-southeast-1", "sa-east-1"}
    case "gcp":
        return []string{"us-east1", "us-west1", "europe-west1", "asia-southeast1"}
    case "azure":
        return []string{"eastus", "westus", "northeurope", "southeastasia"}
    default:
        return []string{"us-east-1"}
    }
}​​​​​​​​​​​​​​​​
