package main

import (
    "fmt"
    "os"
    "strings"
    "time"

    "gopkg.in/yaml.v3"
)

// SovereignConfigFile is the structure of the sovereign YAML config
// file accepted via --config. All fields are optional -- flags
// override config file values when both are present.
//
// The config file allows repeatable runs without long flag strings.
// Store it alongside your lemmings plan file and commit both to
// version control for reproducible load test definitions.
type SovereignConfigFile struct {
    Hit              string        `yaml:"Hit"`
    Planets          int           `yaml:"Planets"`
    Terrain          int64         `yaml:"Terrain"`
    Pack             int64         `yaml:"Pack"`
    Until            string        `yaml:"Until"`
    Ramp             string        `yaml:"Ramp"`
    Plan             string        `yaml:"Plan"`
    Regions          []string      `yaml:"Regions"`
    InstanceType     string        `yaml:"InstanceType"`
    SaveTo           []string      `yaml:"SaveTo"`
    ProvisionTimeout string        `yaml:"ProvisionTimeout"`
    TerminateTimeout string        `yaml:"TerminateTimeout"`
    Yes              bool          `yaml:"Yes"`
    DryRun           bool          `yaml:"DryRun"`
}

// loadConfigFile reads a sovereign YAML config file from path and
// merges it into cfg. Values already set via flags take precedence
// over values in the config file -- the config file fills gaps,
// it does not override explicit flag values.
//
// Duration fields in the config file use the same Go duration string
// syntax as the flags -- 30s, 5m, 1h30m etc.
//
// Returns an error if the file cannot be read or parsed. A missing
// file when no --config flag was provided is not an error -- the
// caller is responsible for only calling this when cfg.ConfigFile
// is non-empty.
func loadConfigFile(path string, cfg *SovereignConfig) error {
    if path == "" {
        return nil
    }

    contents, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("config file: read %s: %w", path, err)
    }

    var file SovereignConfigFile
    if err := yaml.Unmarshal(contents, &file); err != nil {
        return fmt.Errorf("config file: parse %s: %w", path, err)
    }

    if err := mergeConfigFile(&file, cfg); err != nil {
        return fmt.Errorf("config file: merge: %w", err)
    }

    return nil
}

// mergeConfigFile applies non-zero values from file into cfg
// without overwriting values that were explicitly set via flags.
//
// Zero values in cfg indicate the flag was not set and the config
// file value should be used. Non-zero values in cfg indicate an
// explicit flag was provided and must not be overwritten.
func mergeConfigFile(file *SovereignConfigFile, cfg *SovereignConfig) error {
    if cfg.Hit == defaultHit && file.Hit != "" {
        cfg.Hit = file.Hit
    }
    if cfg.Planets == defaultPlanets && file.Planets > 0 {
        cfg.Planets = file.Planets
    }
    if cfg.Terrain == defaultTerrain && file.Terrain > 0 {
        cfg.Terrain = file.Terrain
    }
    if cfg.Pack == defaultPack && file.Pack > 0 {
        cfg.Pack = file.Pack
    }
    if cfg.Plan == "" && file.Plan != "" {
        cfg.Plan = file.Plan
    }
    if cfg.InstanceType == "" && file.InstanceType != "" {
        cfg.InstanceType = file.InstanceType
    }
    if len(cfg.Regions) == 0 && len(file.Regions) > 0 {
        cfg.Regions = file.Regions
    }
    if len(cfg.SaveTo) == 1 && cfg.SaveTo[0] == defaultSaveTo && len(file.SaveTo) > 0 {
        cfg.SaveTo = file.SaveTo
    }
    if !cfg.Yes && file.Yes {
        cfg.Yes = file.Yes
    }
    if !cfg.DryRun && file.DryRun {
        cfg.DryRun = file.DryRun
    }

    // Parse duration fields from config file
    if cfg.Until == defaultUntil && file.Until != "" {
        d, err := time.ParseDuration(strings.TrimSpace(file.Until))
        if err != nil {
            return fmt.Errorf("invalid Until duration %q: %w", file.Until, err)
        }
        cfg.Until = d
    }
    if cfg.Ramp == defaultRamp && file.Ramp != "" {
        d, err := time.ParseDuration(strings.TrimSpace(file.Ramp))
        if err != nil {
            return fmt.Errorf("invalid Ramp duration %q: %w", file.Ramp, err)
        }
        cfg.Ramp = d
    }
    if cfg.ProvisionTimeout == defaultProvisionTimeout && file.ProvisionTimeout != "" {
        d, err := time.ParseDuration(strings.TrimSpace(file.ProvisionTimeout))
        if err != nil {
            return fmt.Errorf("invalid ProvisionTimeout duration %q: %w",
                file.ProvisionTimeout, err)
        }
        cfg.ProvisionTimeout = d
    }
    if cfg.TerminateTimeout == defaultTerminateTimeout && file.TerminateTimeout != "" {
        d, err := time.ParseDuration(strings.TrimSpace(file.TerminateTimeout))
        if err != nil {
            return fmt.Errorf("invalid TerminateTimeout duration %q: %w",
                file.TerminateTimeout, err)
        }
        cfg.TerminateTimeout = d
    }

    return nil
}

// validateConfig checks that the final merged SovereignConfig is
// coherent before NewSovereign is called. It enforces constraints
// that figtree validators cannot express because they depend on
// the relationship between multiple fields.
func validateConfig(cfg *SovereignConfig) error {
    if cfg.Hit == "" {
        return fmt.Errorf("sovereign: --hit is required")
    }
    if !strings.HasPrefix(cfg.Hit, "http") {
        return fmt.Errorf("sovereign: --hit must begin with http or https, got %q",
            cfg.Hit)
    }
    if cfg.Planets < 1 {
        return fmt.Errorf("sovereign: --planets must be at least 1, got %d",
            cfg.Planets)
    }
    if cfg.Terrain < 1 {
        return fmt.Errorf("sovereign: --terrain must be at least 1, got %d",
            cfg.Terrain)
    }
    if cfg.Pack < 1 {
        return fmt.Errorf("sovereign: --pack must be at least 1, got %d",
            cfg.Pack)
    }
    if cfg.Until <= 0 {
        return fmt.Errorf("sovereign: --until must be greater than zero")
    }
    if cfg.Ramp <= 0 {
        return fmt.Errorf("sovereign: --ramp must be greater than zero")
    }
    if cfg.ProvisionTimeout <= 0 {
        return fmt.Errorf("sovereign: --provision-timeout must be greater than zero")
    }
    if cfg.TerminateTimeout <= 0 {
        return fmt.Errorf("sovereign: --terminate-timeout must be greater than zero")
    }
    if cfg.Plan != "" {
        if _, err := os.Stat(cfg.Plan); os.IsNotExist(err) {
            return fmt.Errorf("sovereign: plan file not found: %s", cfg.Plan)
        }
    }
    for _, dest := range cfg.SaveTo {
        if strings.TrimSpace(dest) == "" {
            return fmt.Errorf("sovereign: save-to contains empty destination")
        }
    }
    return nil
}

// exampleConfigFile returns the content of a sovereign.yml example
// for use in documentation and --help output.
func exampleConfigFile() string {
    return `# sovereign.yml
# All flags can be expressed here. Flags override config file values.

Hit: https://myapp.com
Planets: 5
Terrain: 50
Pack: 50
Until: 30s
Ramp: 5m
Plan: lemmings.plan.yml

Regions:
  - us-east-1
  - us-west-1
  - eu-west-1
  - ap-southeast-1
  - sa-east-1

InstanceType: t3.medium

ProvisionTimeout: 5m
TerminateTimeout: 5m

SaveTo:
  - .
  - s3://my-bucket/sovereign-reports
`
}​​​​​​​​​​​​​​​​
