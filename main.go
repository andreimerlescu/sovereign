package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/andreimerlescu/figtree/v2"
)

const (
    defaultHit              string        = "http://localhost:8080/"
    defaultPlanets          int           = 1
    defaultTerrain          int64         = 50
    defaultPack             int64         = 50
    defaultUntil            time.Duration = 30 * time.Second
    defaultRamp             time.Duration = 5 * time.Minute
    defaultProvider         string        = "digitalocean"
    defaultProvisionTimeout time.Duration = 5 * time.Minute
    defaultTerminateTimeout time.Duration = 5 * time.Minute
    defaultYes              bool          = false
    defaultDryRun           bool          = false
    defaultSaveTo           string        = "."

    version = "0.0.1"
)

func main() {
    ctx, cancel := signal.NotifyContext(
        context.Background(),
        os.Interrupt,
        syscall.SIGTERM,
    )
    defer cancel()

    figs := figtree.Grow()

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

    figs.NewString("provider", defaultProvider, "Cloud provider: digitalocean, aws, gcp, azure").
        WithAlias("provider", "pv")

    figs.NewString("plan", "", "Path to a lemmings plan YAML file").
        WithAlias("plan", "pl")

    figs.NewList("region", []string{}, "Comma-separated list of regions to deploy into").
        WithAlias("region", "rg")

    figs.NewString("instance", "", "Instance type per planet").
        WithAlias("instance", "i")

    figs.NewList("save-to", []string{defaultSaveTo}, "Report delivery destinations").
        WithAlias("save-to", "s")

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

    figs.NewString("config", "", "Path to a sovereign YAML config file").
        WithAlias("config", "c")

    if problems := figs.Problems(); len(problems) > 0 {
        for _, p := range problems {
            fmt.Fprintln(os.Stderr, "config error:", p)
        }
        os.Exit(1)
    }

    if err := figs.Load(); err != nil {
        log.Fatalf("failed to load configuration: %v", err)
    }

    cfg := SovereignConfig{
        Hit:              *figs.String("hit"),
        Planets:          *figs.Int("planets"),
        Terrain:          *figs.Int64("terrain"),
        Pack:             *figs.Int64("pack"),
        Until:            *figs.Duration("until"),
        Ramp:             *figs.Duration("ramp"),
        Provider:         *figs.String("provider"),
        Plan:             *figs.String("plan"),
        Regions:          *figs.List("region"),
        InstanceType:     *figs.String("instance"),
        SaveTo:           *figs.List("save-to"),
        ProvisionTimeout: *figs.Duration("provision-timeout"),
        TerminateTimeout: *figs.Duration("terminate-timeout"),
        Yes:              *figs.Bool("yes"),
        DryRun:           *figs.Bool("dry-run"),
        ConfigFile:       *figs.String("config"),
        Version:          version,
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

func printBootSummary(cfg SovereignConfig) {
    fmt.Printf("\nsovereign v%s\n", cfg.Version)
    fmt.Println("─────────────────────────────────────────")
    fmt.Printf("  target:    %s\n", cfg.Hit)
    fmt.Printf("  planets:   %d\n", cfg.Planets)
    fmt.Printf("  terrain:   %d groups per planet\n", cfg.Terrain)
    fmt.Printf("  pack:      %d lemmings per terrain\n", cfg.Pack)
    fmt.Printf("  until:     %s per lemming\n", cfg.Until)
    fmt.Printf("  ramp:      %s to full concurrency\n", cfg.Ramp)
    fmt.Printf("  provider:  %s\n", cfg.Provider)
    if len(cfg.Regions) > 0 {
        fmt.Printf("  regions:\n")
        for _, r := range cfg.Regions {
            fmt.Printf("    -- %s\n", r)
        }
    }
    if cfg.Plan != "" {
        fmt.Printf("  plan:      %s\n", cfg.Plan)
    }
    fmt.Printf("  save-to:\n")
    for _, dest := range cfg.SaveTo {
        fmt.Printf("    -- %s\n", dest)
    }
    if cfg.DryRun {
        fmt.Printf("\n  dry-run:   cost estimate only, no provisioning\n")
    }
    fmt.Println("─────────────────────────────────────────")
    fmt.Println("  sovereign is preparing terrain...")
    fmt.Println()
}​​​​​​​​​​​​​​​​
