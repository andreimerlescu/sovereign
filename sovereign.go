package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/commander"
	"github.com/andreimerlescu/sovereign/pkg/events"
	"github.com/andreimerlescu/sovereign/pkg/ledger"
	"github.com/andreimerlescu/sovereign/pkg/pricing"
	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform/provider"
	tfaws "github.com/andreimerlescu/sovereign/pkg/terraform/provider/aws"
	tfdo "github.com/andreimerlescu/sovereign/pkg/terraform/provider/do"
	tfovh "github.com/andreimerlescu/sovereign/pkg/terraform/provider/ovh"
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
	Provider string
	Regions  []string
	SaveTo   []string
	// SaveToKey is the SSH private key path used when a save-to destination
	// uses the ssh:// scheme. If empty the SSH deliverer omits -i and
	// relies on the SSH agent.
	SaveToKey string

	// SSHOptions are extra OpenSSH -o arguments passed to every ssh and
	// scp invocation sovereign makes. They are appended after the built-in
	// StrictHostKeyChecking=no and ConnectTimeout=30 options, so they may
	// override the defaults if needed. Example:
	//   []string{"UserKnownHostsFile=/path/to/known_hosts", "StrictHostKeyChecking=yes"}
	SSHOptions []string

	// FTPUsername and FTPPassword are credentials for ftp:// save-to destinations.
	FTPUsername string
	FTPPassword string

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

	// AMI / image selection
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

var defaultAWSRegions = []string{
	"us-east-1",
	"us-west-1",
	"eu-west-1",
	"ap-southeast-1",
	"sa-east-1",
}

var defaultDORegions = []string{
	"nyc1",
	"sfo3",
	"ams3",
	"sgp1",
	"lon1",
}

var defaultOVHRegions = []string{
	"GRA11",
	"SBG5",
	"DE1",
	"UK1",
	"SGP1",
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

// SovereignState is persisted to disk so sovereign can resume after
// a crash without orphaning infrastructure.
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
	ledger    *ledger.Ledger
	bus       *events.EventBus
	cmd       commander.Commander
	out       io.Writer
}

// NewSovereign constructs a Sovereign, initializes the provider, event
// bus, and ledger, loads any existing state, and prepares for Run.
func NewSovereign(ctx context.Context, cfg SovereignConfig) (*Sovereign, error) {
	ctx, cancel := context.WithCancel(ctx)

	s := &Sovereign{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		startedAt: time.Now(),
		out:       os.Stdout,
	}
	s.cmd = &commander.RealCommander{}

	stateFile, err := s.resolveStateFile()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sovereign: resolve state file: %w", err)
	}
	s.stateFile = stateFile

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

	s.bus = events.NewEventBus()

	home, err := os.UserHomeDir()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sovereign: resolve home dir: %w", err)
	}
	sovereignDir := filepath.Join(home, ".sovereignd")
	l, err := ledger.NewLedger(sovereignDir, s.startedAt)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("sovereign: initialize ledger: %w", err)
	}
	s.ledger = l
	s.bus.Subscribe(s.ledger.AsSubscriber())

	existing, loadErr := s.loadState()
	if loadErr == nil && existing != nil {
		fmt.Fprintf(s.out, "  sovereign: resuming from existing state at %s\n", stateFile)
		s.bus.Emit(events.Event{
			Kind:   events.KindRunResumed,
			Detail: stateFile,
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
//
// Destroy-on-failure policy:
//   - Step 4 (provisioning): if provisionTerrain returns an error, any
//     planets already marked PlanetStatusRunning are destroyed before
//     returning. This prevents orphaned instances when planet 1 succeeds
//     and planet 2+ fail.
//   - Steps 6, 7, 8 (lemmings, collection, report): destroyTerrain is
//     called before returning the error.
//   - Step 9 (destroy): errors are returned directly; the caller must
//     handle manual cleanup using the instance IDs logged to stderr.
func (s *Sovereign) Run() error {
	defer s.cancel()
	defer s.bus.Close()

	estimate, err := s.estimateCost()
	if err != nil {
		return fmt.Errorf("sovereign: cost estimation: %w", err)
	}
	s.printCostEstimate(estimate)

	if s.cfg.DryRun {
		fmt.Fprintln(s.out, "  sovereign: dry-run complete -- no infrastructure provisioned")
		return nil
	}

	if !s.cfg.Yes {
		if err := s.confirmWithUser(); err != nil {
			return fmt.Errorf("sovereign: aborted by user")
		}
	}

	fmt.Fprintln(s.out, "  sovereign: provisioning terrain...")
	if err := s.provisionTerrain(); err != nil {
		// Destroy any planets that were already provisioned before the
		// error occurred. Errors from destroy are secondary -- log them
		// but return the original provisioning error.
		if destroyErr := s.destroyTerrain(); destroyErr != nil {
			fmt.Fprintf(os.Stderr, "  sovereign: additionally failed to destroy terrain: %v\n", destroyErr)
		}
		return fmt.Errorf("sovereign: provision terrain: %w", err)
	}

	if err := s.saveState(); err != nil {
		fmt.Fprintf(s.out, "  sovereign: warning: failed to save state: %v\n", err)
	}

	fmt.Fprintln(s.out, "  sovereign: running lemmings on planets...")
	if err := s.runLemmings(); err != nil {
		_ = s.destroyTerrain()
		if errors.Is(err, ErrAllPlanetsLost) {
			return fmt.Errorf("sovereign: all planets lost during lemmings execution -- terrain destroyed")
		}
		return fmt.Errorf("sovereign: run lemmings: %w", err)
	}

	fmt.Fprintln(s.out, "  sovereign: collecting results from planets...")
	if err := s.collectResults(); err != nil {
		_ = s.destroyTerrain()
		return fmt.Errorf("sovereign: collect results: %w", err)
	}

	fmt.Fprintln(s.out, "  sovereign: aggregating unified report...")
	if err := s.aggregateReport(); err != nil {
		_ = s.destroyTerrain()
		return fmt.Errorf("sovereign: aggregate report: %w", err)
	}

	fmt.Fprintln(s.out, "  sovereign: destroying terrain...")
	if err := s.destroyTerrain(); err != nil {
		return fmt.Errorf("sovereign: destroy terrain: %w", err)
	}

	s.cleanState()
	s.printFinalSummary()

	return nil
}

// initPlanets builds the initial planet list from config.
func (s *Sovereign) initPlanets() []*Planet {
	regions := s.cfg.Regions
	if len(regions) == 0 {
		regions = s.defaultRegions()
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

// defaultRegions returns the appropriate default region list for the
// configured provider.
func (s *Sovereign) defaultRegions() []string {
	switch s.cfg.Provider {
	case "digitalocean":
		return defaultDORegions
	case "ovh":
		return defaultOVHRegions
	default:
		return defaultAWSRegions
	}
}

// confirmWithUser prompts the engineer before provisioning.
func (s *Sovereign) confirmWithUser() error {
	fmt.Fprint(s.out, "  sovereign: proceed with provisioning? [y/N] ")
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil || (response != "y" && response != "Y") {
		return fmt.Errorf("aborted")
	}
	return nil
}

// printCostEstimate prints the projected cost before provisioning.
func (s *Sovereign) printCostEstimate(estimate *CostEstimate) {
	fmt.Fprintln(s.out)
	fmt.Fprintln(s.out, "  sovereign: projected cost estimate (via instances.vantage.sh)")
	fmt.Fprintln(s.out, "  ──────────────────────────────────────────────────────────────")
	for _, p := range estimate.Planets {
		fmt.Fprintf(s.out, "    planet %d  %s  %s  %s/hr\n",
			p.ID, p.Region, p.InstanceType,
			pricing.MicrocentsToDetailedDisplay(p.HourlyRate))
	}
	fmt.Fprintf(s.out, "\n    total planets:   %d\n", len(estimate.Planets))
	fmt.Fprintf(s.out, "    est. duration:   %s\n", estimate.Duration.Round(time.Second))
	fmt.Fprintf(s.out, "    projected cost:  %s\n", pricing.MicrocentsToDisplay(estimate.TotalCost))
	fmt.Fprintln(s.out)
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
	fmt.Fprintln(s.out)
	fmt.Fprintln(s.out, "─────────────────────────────────────────")
	fmt.Fprintf(s.out, "  sovereign v%s -- final summary\n", s.cfg.Version)
	fmt.Fprintln(s.out, "─────────────────────────────────────────")
	fmt.Fprintf(s.out, "  target:    %s\n", s.cfg.Hit)
	fmt.Fprintf(s.out, "  provider:  %s\n", s.cfg.Provider)
	fmt.Fprintf(s.out, "  planets:   %d provisioned\n", len(s.planets))
	fmt.Fprintf(s.out, "  completed: %d\n", done)
	if lost > 0 {
		fmt.Fprintf(s.out, "  lost:      %d (manual intervention required)\n", lost)
	}
	fmt.Fprintf(s.out, "  ledger:    %s\n", s.ledger.Path())
	fmt.Fprintf(s.out, "  elapsed:   %s\n", elapsed)
	fmt.Fprintln(s.out, "─────────────────────────────────────────")
}

// newProvider constructs the TerraformProvider for the configured
// cloud provider.
func newProvider(cfg SovereignConfig) (tfprovider.TerraformProvider, error) {
	switch cfg.Provider {
	case "aws":
		return tfaws.New()
	case "digitalocean":
		return tfdo.New()
	case "ovh":
		return tfovh.New()
	default:
		return nil, fmt.Errorf("sovereign: unknown provider %q -- supported: aws, digitalocean, ovh",
			cfg.Provider)
	}
}
