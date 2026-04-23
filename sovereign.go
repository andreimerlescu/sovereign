package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
	tfaws "github.com/andreimerlescu/sovereign/pkg/terraform_provider/aws"
)

// SovereignConfig is the single source of truth flowing from CLI
// or config file into every layer of sovereign.
type SovereignConfig struct {
	// Core lemmings parameters
	Hit     string
	Planets int
	Terrain int64
	Pack    int64
	Until   time.Duration
	Ramp    time.Duration
	Plan    string

	// Infrastructure
	Provider  string
	Regions   []string
	SaveTo    []string

	// Timeouts
	ProvisionTimeout time.Duration
	TerminateTimeout time.Duration

	// Confirmation
	Yes    bool
	DryRun bool

	// Config file
	ConfigFile string

	// Version
	Version string

	// Remote instance
	RemoteUser string

	// AMI selection
	AMIOS     string
	AMIOwners string

	// Instance type per planet
	InstanceType string

	// Go and lemmings installation
	GoVersion       string
	LemmingsPackage string

	// EBS volumes
	RootVolumeSize int
	RootVolumeType string
	TempVolumeSize int
	TempVolumeType string
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

// defaultAWSRegions is used when no regions are specified.
// Five regions providing geographic distribution across
// North America, Europe, and Asia Pacific.
var defaultAWSRegions = []string{
	"us-east-1",
	"us-west-1",
	"eu-west-1",
	"ap-southeast-1",
	"sa-east-1",
}

// Planet represents a single remote instance running lemmings.
type Planet struct {
	ID            int          `yaml:"ID"`
	Region        string       `yaml:"Region"`
	InstanceType  string       `yaml:"InstanceType"`
	RemoteUser    string       `yaml:"RemoteUser"`
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
// after a crash without orphaning infrastructure.
type SovereignState struct {
	Version   string          `yaml:"Version"`
	StartedAt time.Time       `yaml:"StartedAt"`
	Config    SovereignConfig `yaml:"Config"`
	Planets   []*Planet       `yaml:"Planets"`
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
	provider  tfprovider.TerraformProvider
	ledger    *Ledger
}

// NewSovereign constructs a Sovereign, initializes the provider and
// ledger, loads any existing state, and prepares for Run.
func NewSovereign(ctx context.Context, cfg SovereignConfig) (*Sovereign, error) {
	ctx, cancel := context.WithCancel(ctx)

	s := &Sovereign{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		startedAt: time.Now(),
	}

	// resolve state file
	stateFile, err := s.resolveStateFile()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sovereign: resolve state file: %w", err)
	}
	s.stateFile = stateFile

	// initialize provider
	provider, err := newProvider(cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sovereign: initialize provider: %w", err)
	}
	if err := provider.Validate(); err != nil {
		cancel()
		return nil, fmt.Errorf("sovereign: validate provider: %w", err)
	}
	s.provider = provider

	// initialize ledger
	home, err := os.UserHomeDir()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sovereign: resolve home dir: %w", err)
	}
	sovereignDir := filepath.Join(home, ".sovereignd")
	ledger, err := NewLedger(sovereignDir, s.startedAt)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sovereign: initialize ledger: %w", err)
	}
	s.ledger = ledger

	// load existing state or initialize fresh
	existing, loadErr := s.loadState()
	if loadErr == nil && existing != nil {
		fmt.Printf("  sovereign: resuming from existing state at %s\n", stateFile)
		s.ledger.Append(LedgerEvent{
			EventType: LedgerEventResumed,
			Detail:    stateFile,
		})
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
func (s *Sovereign) Run() error {
	defer s.cancel()

	// Step 1 -- estimate cost
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

	// Step 4 -- provision terrain
	fmt.Println("  sovereign: provisioning terrain...")
	if err := s.provisionTerrain(); err != nil {
		return fmt.Errorf("sovereign: provision terrain: %w", err)
	}

	// Step 5 -- persist state for crash recovery
	if err := s.saveState(); err != nil {
		fmt.Printf("  sovereign: warning: failed to save state: %v\n", err)
	}

	// Step 6 -- run lemmings on each planet
	fmt.Println("  sovereign: running lemmings on planets...")
	if err := s.runLemmings(); err != nil {
		_ = s.destroyTerrain()
		return fmt.Errorf("sovereign: run lemmings: %w", err)
	}

	// Step 7 -- collect results from each planet
	fmt.Println("  sovereign: collecting results from planets...")
	if err := s.collectResults(); err != nil {
		_ = s.destroyTerrain()
		return fmt.Errorf("sovereign: collect results: %w", err)
	}

	// Step 8 -- aggregate unified report
	fmt.Println("  sovereign: aggregating unified report...")
	if err := s.aggregateReport(); err != nil {
		_ = s.destroyTerrain()
		return fmt.Errorf("sovereign: aggregate report: %w", err)
	}

	// Step 9 -- destroy terrain
	fmt.Println("  sovereign: destroying terrain...")
	if err := s.destroyTerrain(); err != nil {
		return fmt.Errorf("sovereign: destroy terrain: %w", err)
	}

	// Step 10 -- clean up state file on successful completion
	s.cleanState()

	s.printFinalSummary()

	return nil
}

// initPlanets builds the initial planet list from config.
// Regions are distributed round-robin across the requested planet count.
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
			RemoteUser:   s.cfg.RemoteUser, 
			Status:       PlanetStatusPending,
		}
	}
	return planets
}

// confirmWithUser prompts the engineer before provisioning.
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
	fmt.Println("  sovereign: projected cost estimate (via instances.vantage.sh)")
	fmt.Println("  ──────────────────────────────────────────────────────────────")
	for _, p := range estimate.Planets {
		fmt.Printf("    planet %d  %s  %s  %s/hr\n",
			p.ID, p.Region, p.InstanceType,
			microcentsToDetailedDisplay(p.HourlyRate))
	}
	fmt.Printf("\n    total planets:   %d\n", len(estimate.Planets))
	fmt.Printf("    est. duration:   %s\n", estimate.Duration.Round(time.Second))
	fmt.Printf("    projected cost:  %s\n", microcentsToDisplay(estimate.TotalCost))
	fmt.Println()
}

// printFinalSummary writes the post-run summary to stdout.
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
	fmt.Printf("  ledger:    %s\n", s.ledger.Path())
	fmt.Printf("  elapsed:   %s\n", elapsed)
	fmt.Println("─────────────────────────────────────────")
}

// newProvider constructs the TerraformProvider for the configured
// cloud provider. Additional providers are added here as they are
// implemented.
func newProvider(cfg SovereignConfig) (tfprovider.TerraformProvider, error) {
	switch cfg.Provider {
	case "aws":
		return tfaws.New()
	default:
		return nil, fmt.Errorf("sovereign: unknown provider %q -- supported: aws",
			cfg.Provider)
	}
}
