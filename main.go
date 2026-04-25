package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/andreimerlescu/figtree/v2"
)

const (
	// Core defaults
	defaultHit     string        = "http://localhost:8080/"
	defaultPlanets int           = 1
	defaultTerrain int64         = 50
	defaultPack    int64         = 50
	defaultUntil   time.Duration = 30 * time.Second
	defaultRamp    time.Duration = 5 * time.Minute

	// Infrastructure defaults
	defaultProvider         string        = "aws"
	defaultProvisionTimeout time.Duration = 5 * time.Minute
	defaultTerminateTimeout time.Duration = 5 * time.Minute
	defaultYes              bool          = false
	defaultDryRun           bool          = false
	defaultSaveTo           string        = "."

	// Remote instance defaults
	defaultRemoteUser string = "rocky"

	// AMI defaults
	defaultAMIOS     string = "Rocky-9-EC2-Base-9.*-x86_64*"
	defaultAMIOwners string = "792107900819"

	// Go and lemmings defaults
	defaultGoVersion       string = "1.25.0"
	defaultLemmingsPackage string = "github.com/andreimerlescu/lemmings@latest"

	// EBS volume defaults
	defaultRootVolumeSize int    = 30
	defaultRootVolumeType string = "gp2"
	defaultTempVolumeSize int    = 10
	defaultTempVolumeType string = "gp2"
)

//go:embed VERSION
var versionBytes []byte // same as englishBytes pattern above

func Version() string {
	return strings.TrimSpace(string(versionBytes))
}

func main() {
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	figs := figtree.Grow()

	// -- Core ----------------------------------------------------------------

	figs.NewString("hit", defaultHit, "Target URL for lemmings to load test").
		WithAlias("hit", "h").
		WithValidator("hit", figtree.AssureStringNotEmpty).
		WithValidator("hit", figtree.AssureStringHasPrefix("http"))

	figs.NewInt("planets", defaultPlanets, "Number of cloud instances to provision").
		WithAlias("planets", "p").
		WithValidator("planets", figtree.AssureIntInRange(1, 1000))

	figs.NewInt64("terrain", defaultTerrain, "Number of terrain groups per planet").
		WithAlias("terrain", "t").
		WithValidator("terrain", figtree.AssureInt64InRange(1, 10_000))

	figs.NewInt64("pack", defaultPack, "Number of lemmings per terrain per planet").
		WithAlias("pack", "pk").
		WithValidator("pack", figtree.AssureInt64InRange(1, 100_000))

	figs.NewDuration("until", defaultUntil, "How long each lemming lives").
		WithAlias("until", "u").
		WithValidator("until", figtree.AssureDurationGreaterThan(0)).
		WithValidator("until", figtree.AssureDurationLessThan(24*time.Hour))

	figs.NewDuration("ramp", defaultRamp, "Duration to bring all terrains online per planet").
		WithAlias("ramp", "r").
		WithValidator("ramp", figtree.AssureDurationGreaterThan(0)).
		WithValidator("ramp", figtree.AssureDurationLessThan(24*time.Hour))

	figs.NewString("plan", "", "Path to a lemmings plan YAML file").
		WithAlias("plan", "pl")

	figs.NewList("save-to", []string{defaultSaveTo}, "Report delivery destinations").
		WithAlias("save-to", "s")

	figs.NewString("save-to-key", "", "SSH private key path for ssh:// save-to destinations").
		WithAlias("save-to-key", "sk")

	// -- Infrastructure ------------------------------------------------------

	figs.NewString("provider", defaultProvider, "Cloud provider: aws").
		WithAlias("provider", "pv")

	figs.NewList("region", []string{}, "AWS regions to deploy into").
		WithAlias("region", "rg")

	figs.NewString("instance", "", "Instance type per planet (e.g. t3.medium, c6a.large)").
		WithAlias("instance", "i")

	figs.NewDuration("provision-timeout", defaultProvisionTimeout,
		"Maximum time to provision one planet").
		WithAlias("provision-timeout", "pt")

	figs.NewDuration("terminate-timeout", defaultTerminateTimeout,
		"Maximum time to terminate one planet").
		WithAlias("terminate-timeout", "tt")

	figs.NewBool("yes", defaultYes, "Skip cost confirmation prompt").
		WithAlias("yes", "y")

	figs.NewBool("dry-run", defaultDryRun, "Estimate cost and exit without provisioning").
		WithAlias("dry-run", "dr")

	// -- Remote instance -----------------------------------------------------

	figs.NewString("remote-user", defaultRemoteUser,
		"SSH user on provisioned instances").
		WithAlias("remote-user", "ru")

	// -- AMI -----------------------------------------------------------------

	figs.NewString("ami-os", defaultAMIOS,
		"AMI OS search pattern for AWS AMI lookup").
		WithAlias("ami-os", "ao")

	figs.NewString("ami-owners", defaultAMIOwners,
		"AMI owners account ID for AWS AMI lookup").
		WithAlias("ami-owners", "aw")

	// -- Go and lemmings -----------------------------------------------------

	figs.NewString("go-version", defaultGoVersion,
		"Go version to install on planet instances via igo").
		WithAlias("go-version", "gv")

	figs.NewString("lemmings-package", defaultLemmingsPackage,
		"Lemmings Go package path installed via go install").
		WithAlias("lemmings-package", "lp")

	// -- EBS volumes ---------------------------------------------------------

	figs.NewInt("root-volume-size", defaultRootVolumeSize,
		"Root EBS volume size in GB").
		WithAlias("root-volume-size", "rvs")

	figs.NewString("root-volume-type", defaultRootVolumeType,
		"Root EBS volume type (gp2, gp3, io2)").
		WithAlias("root-volume-type", "rvt")

	figs.NewInt("temp-volume-size", defaultTempVolumeSize,
		"Temp EBS volume size in GB").
		WithAlias("temp-volume-size", "tvs")

	figs.NewString("temp-volume-type", defaultTempVolumeType,
		"Temp EBS volume type (gp2, gp3, io2)").
		WithAlias("temp-volume-type", "tvt")

	// -- Config file ---------------------------------------------------------

	figs.NewString("config", "", "Path to a sovereign YAML config file").
		WithAlias("config", "c")

	figs.NewBool("version", false, "Show version").WithAlias("version", "v")

	// -- Validate and load ---------------------------------------------------

	if problems := figs.Problems(); len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "config error:", p)
		}
		os.Exit(1)
	}

	if err := figs.Load(); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	if *figs.Bool("version") {
		fmt.Printf("sovereign %s\n", Version())
		os.Exit(0)
	}

	cfg := SovereignConfig{
		// Core
		Hit:       *figs.String("hit"),
		Planets:   *figs.Int("planets"),
		Terrain:   *figs.Int64("terrain"),
		Pack:      *figs.Int64("pack"),
		Until:     *figs.Duration("until"),
		Ramp:      *figs.Duration("ramp"),
		Plan:      *figs.String("plan"),
		SaveTo:    *figs.List("save-to"),
		SaveToKey: *figs.String("save-to-key"),

		// Infrastructure
		Provider:         *figs.String("provider"),
		Regions:          *figs.List("region"),
		InstanceType:     *figs.String("instance"),
		ProvisionTimeout: *figs.Duration("provision-timeout"),
		TerminateTimeout: *figs.Duration("terminate-timeout"),
		Yes:              *figs.Bool("yes"),
		DryRun:           *figs.Bool("dry-run"),

		// Remote instance
		RemoteUser: *figs.String("remote-user"),

		// AMI
		AMIOS:     *figs.String("ami-os"),
		AMIOwners: *figs.String("ami-owners"),

		// Go and lemmings
		GoVersion:       *figs.String("go-version"),
		LemmingsPackage: *figs.String("lemmings-package"),

		// EBS volumes
		RootVolumeSize: *figs.Int("root-volume-size"),
		RootVolumeType: *figs.String("root-volume-type"),
		TempVolumeSize: *figs.Int("temp-volume-size"),
		TempVolumeType: *figs.String("temp-volume-type"),

		// Config file
		ConfigFile: *figs.String("config"),
		Version:    Version(),
	}

	// load config file if provided -- flags take precedence
	if cfg.ConfigFile != "" {
		if err := loadConfigFile(cfg.ConfigFile, &cfg); err != nil {
			log.Fatalf("failed to load config file: %v", err)
		}
	}

	if err := validateConfig(&cfg); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	printBootSummary(cfg)

	s, err := NewSovereign(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to initialize sovereign: %v", err)
	}

	if err := s.Run(); err != nil {
		log.Fatalf("sovereign error: %v", err)
	}
}

// printBootSummary prints the operator-visible configuration summary
// before sovereign begins any work.
func printBootSummary(cfg SovereignConfig) {
	fmt.Printf("\nsovereign v%s\n", cfg.Version)
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("  target:          %s\n", cfg.Hit)
	fmt.Printf("  planets:         %d\n", cfg.Planets)
	fmt.Printf("  terrain:         %d groups per planet\n", cfg.Terrain)
	fmt.Printf("  pack:            %d lemmings per terrain\n", cfg.Pack)
	fmt.Printf("  until:           %s per lemming\n", cfg.Until)
	fmt.Printf("  ramp:            %s to full concurrency\n", cfg.Ramp)
	fmt.Printf("  provider:        %s\n", cfg.Provider)
	fmt.Printf("  instance:        %s\n", cfg.InstanceType)
	fmt.Printf("  remote-user:     %s\n", cfg.RemoteUser)
	fmt.Printf("  ami-os:          %s\n", cfg.AMIOS)
	fmt.Printf("  ami-owners:      %s\n", cfg.AMIOwners)
	fmt.Printf("  go-version:      %s\n", cfg.GoVersion)
	fmt.Printf("  lemmings:        %s\n", cfg.LemmingsPackage)
	fmt.Printf("  root-volume:     %dGB %s\n", cfg.RootVolumeSize, cfg.RootVolumeType)
	fmt.Printf("  temp-volume:     %dGB %s\n", cfg.TempVolumeSize, cfg.TempVolumeType)
	if len(cfg.Regions) > 0 {
		fmt.Printf("  regions:\n")
		for _, r := range cfg.Regions {
			fmt.Printf("    -- %s\n", r)
		}
	}
	if cfg.Plan != "" {
		fmt.Printf("  plan:            %s\n", cfg.Plan)
	}
	fmt.Printf("  save-to:\n")
	for _, dest := range cfg.SaveTo {
		fmt.Printf("    -- %s\n", dest)
	}
	if cfg.DryRun {
		fmt.Printf("\n  dry-run:         cost estimate only, no provisioning\n")
	}
	fmt.Println("─────────────────────────────────────────")
	fmt.Println("  sovereign is preparing terrain...")
	fmt.Println()
}
