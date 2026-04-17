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
//
// Provider is intentionally absent. Sovereign uses topobuilder for
// infrastructure provisioning and topobuilder currently supports AWS
// only. When topobuilder gains additional provider support, sovereign
// gains it automatically. Provider selection is not sovereign's concern.
type SovereignConfig struct {
    Hit              string
    Planets          int
    Terrain          int64
    Pack             int64
    Until            time.Duration
    Ramp             time.Duration
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

// PlanetStatus represents the current lifecycle state of a planet.
type PlanetStatus string

const (
    PlanetStatusPending      PlanetStatus = "pending"
    PlanetStatusProvisioning PlanetStatus = "provisioning"
    PlanetStatusRunning      PlanetStatus = "running"
    PlanetStatusCollecting   PlanetStatus = "collecting"
    PlanetStatusDone         PlanetStatus = "done"
    PlanetStatusLost         PlanetStatus = "lost"
)

// defaultAWSRegions returns the default AWS region list used when
// no regions are specified in the config or via flag.
// These five regions provide geographic distribution across North
// America, Europe, and Asia Pacific.
//
// Sovereign supports AWS only. This is a direct function of
// topobuilder's current provider support. When topobuilder gains
// additional provider support, this list will be extended.
var defaultAWSRegions = []string{
    "us-east-1",
    "us-west-1",
    "eu-west-1",
    "ap-southeast-1",
    "sa-east-1",
}

// Planet represents a single remote AWS instance running lemmings.
// Each planet is provisioned by topobuilder, runs lemmings with the
// sovereign-generated config, and reports its LifeLog back to sovereign
// when its run completes.
type Planet struct {
    ID            int          `yaml:"ID"`
    Region        string       `yaml:"Region"`
    InstanceType  string       `yaml:"InstanceType"`
    PublicIPv4    string       `yaml:"PublicIPv4"`
    InstanceID    string       `yaml:"InstanceID"`
    Status        PlanetStatus `yaml:"Status"`
    ProvisionedAt time.Time    `yaml:"ProvisionedAt"`
    CompletedAt   time.Time    `yaml:"CompletedAt"`
    ReportPath    string       `yaml:"ReportPath"`
    Error         error        `yaml:"-"`
    mu            sync.Mutex   `yaml:"-"`
}

// SovereignState is persisted to disk so sovereign can resume
// after a crash without orphaning AWS infrastructure.
// When sovereign finds an existing state file on startup it
// reconnects to the existing planets rather than provisioning new ones.
type SovereignState struct {
    Version   string          `yaml:"Version"`
    StartedAt time.Time       `yaml:"StartedAt"`
    Config    SovereignConfig `yaml:"Config"`
    Planets   []*Planet       `yaml:"Planets"`
}

// Sovereign is the top level coordinator. One sovereign per invocation.
// It owns the full lifecycle -- cost estimation, terrain provisioning,
// lemmings deployment, result collection, report aggregation, and
// terrain destruction.
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
// disk, and prepares for Run. If an existing state file is found,
// sovereign resumes from that state rather than provisioning fresh
// infrastructure -- this ensures a crash does not orphan AWS resources.
func NewSovereign(ctx context.Context, cfg SovereignConfig) (*Sovereign, error) {
    ctx, cancel := context.WithCancel(ctx)

    s := &Sovereign{
        cfg:       cfg,
        ctx:       ctx,
        cancel:    cancel,
        startedAt: time.Now(),
    }

    stateFile, err := s.resolveStateFile()
    if err != nil {
        cancel()
        return nil, fmt.Errorf("sovereign: resolve state file: %w", err)
    }
    s.stateFile = stateFile

    existing, loadErr := s.loadState()
    if loadErr == nil && existing != nil {
        fmt.Printf("  sovereign: resuming from existing state at %s\n", stateFile)
        s.state = existing
        s.planets = existing.Planets
    } else {
        s.planets = s.initPlanets()
        s.state = &SovereignState{
            Version:   cfg.Version,
            StartedAt: s.startedAt,
            Config:    cfg,
            Planets:   s.planets,
        }
    }

    return s, nil
}

// Run executes the full sovereign lifecycle in order.
// Each step is independently logged. A failure in any step
// after provisioning triggers a destroy attempt before returning
// the error so that AWS resources are not left running.
func (s *Sovereign) Run() error {
    defer s.cancel()

    // Step 1 -- estimate cost via instances.vantage.sh
    estimate, err := s.estimateCost()
    if err != nil {
        return fmt.Errorf("sovereign: cost estimation: %w", err)
    }
    s.printCostEstimate(estimate)

    // Step 2 -- dry run exits after estimate
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

    // Step 4 -- provision AWS terrain via topobuilder
    fmt.Println("  sovereign: provisioning terrain via topobuilder...")
    if err := s.provisionTerrain(); err != nil {
        return fmt.Errorf("sovereign: provision terrain: %w", err)
    }

    // Step 5 -- persist state for crash recovery
    if err := s.saveState(); err != nil {
        fmt.Printf("  sovereign: warning: failed to save state: %v\n", err)
    }

    // Step 6 -- run lemmings on each planet concurrently
    // A failed planet is marked PlanetStatusLost and does not
    // block collection from successful planets.
    fmt.Println("  sovereign: running lemmings on planets...")
    if err := s.runLemmings(); err != nil {
        _ = s.destroyTerrain()
        return fmt.Errorf("sovereign: run lemmings: %w", err)
    }

    // Step 7 -- collect LifeLogs and reports from each planet
    // Collection runs per planet as each one completes.
    fmt.Println("  sovereign: collecting results from planets...")
    if err := s.collectResults(); err != nil {
        _ = s.destroyTerrain()
        return fmt.Errorf("sovereign: collect results: %w", err)
    }

    // Step 8 -- aggregate all planet LifeLogs into unified report
    fmt.Println("  sovereign: aggregating unified report...")
    if err := s.aggregateReport(); err != nil {
        _ = s.destroyTerrain()
        return fmt.Errorf("sovereign: aggregate report: %w", err)
    }

    // Step 9 -- destroy AWS terrain via topobuilder
    fmt.Println("  sovereign: destroying terrain via topobuilder...")
    if err := s.destroyTerrain(); err != nil {
        return fmt.Errorf("sovereign: destroy terrain: %w", err)
    }

    // Step 10 -- clean up state file on successful completion
    s.cleanState()

    s.printFinalSummary()

    return nil
}

// initPlanets builds the initial planet list from config.
// Regions are distributed round-robin across the requested planet
// count. If no regions are specified, defaultAWSRegions is used.
func (s *Sovereign) initPlanets() []*Planet {
    regions := s.cfg.Regions
    if len(regions) == 0 {
        regions = defaultAWSRegions
    }
    planets := make([]*Planet, s.cfg.Planets)
    for i := 0; i < s.cfg.Planets; i++ {
        planets[i] = &Planet{
            ID:           i + 1,
            Region:       regions[i%len(regions)],
            InstanceType: s.cfg.InstanceType,
            Status:       PlanetStatusPending,
        }
    }
    return planets
}

// resolveStateFile determines the state file path.
// Prefers .sovereignd in the current working directory over
// ~/.sovereignd so project-local state is isolated.
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
// Returns nil, nil when no state file exists -- not an error.
func (s *Sovereign) loadState() (*SovereignState, error) {
    if _, err := os.Stat(s.stateFile); os.IsNotExist(err) {
        return nil, nil
    }
    // TODO: unmarshal YAML state file into SovereignState
    return nil, nil
}

// saveState writes current state to disk for crash recovery.
// The state directory is created with mode 0700 if absent.
func (s *Sovereign) saveState() error {
    dir := filepath.Dir(s.stateFile)
    if err := os.MkdirAll(dir, 0700); err != nil {
        return fmt.Errorf("save state mkdir: %w", err)
    }
    // TODO: marshal SovereignState to YAML and write to stateFile
    return nil
}

// cleanState removes the state file after a successful run.
// A missing state file is not an error.
func (s *Sovereign) cleanState() {
    _ = os.Remove(s.stateFile)
}

// confirmWithUser prompts the engineer before provisioning AWS resources.
func (s *Sovereign) confirmWithUser() error {
    fmt.Print("  sovereign: proceed with provisioning? [y/N] ")
    var response string
    _, err := fmt.Scanln(&response)
    if err != nil || (response != "y" && response != "Y") {
        return fmt.Errorf("aborted")
    }
    return nil
}

// printCostEstimate prints the projected AWS cost before provisioning.
func (s *Sovereign) printCostEstimate(estimate *CostEstimate) {
    fmt.Println()
    fmt.Println("  sovereign: projected cost estimate (via instances.vantage.sh)")
    fmt.Println("  ──────────────────────────────────────────────────────────────")
    for _, p := range estimate.Planets {
        fmt.Printf("    planet %d  %s  %s  $%.4f/hr\n",
            p.ID, p.Region, p.InstanceType, p.HourlyRate)
    }
    fmt.Printf("\n    total planets:   %d\n", len(estimate.Planets))
    fmt.Printf("    est. duration:   %s\n", estimate.Duration.Round(time.Second))
    fmt.Printf("    projected cost:  $%.4f\n", estimate.TotalCost)
    fmt.Println()
}

// printFinalSummary writes the post-run block to STDOUT.
func (s *Sovereign) printFinalSummary() {
    elapsed := time.Since(s.startedAt).Round(time.Second)
    done, lost := 0, 0
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
    fmt.Printf("  completed: %d\n", done)
    if lost > 0 {
        fmt.Printf("  lost:      %d (manual intervention required)\n", lost)
    }
    fmt.Printf("  elapsed:   %s\n", elapsed)
    fmt.Println("─────────────────────────────────────────")
}​​​​​​​​​​​​​​​​
