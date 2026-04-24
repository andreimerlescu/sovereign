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
type SovereignConfigFile struct {
	// Core
	Hit     string   `yaml:"Hit"`
	Planets int      `yaml:"Planets"`
	Terrain int64    `yaml:"Terrain"`
	Pack    int64    `yaml:"Pack"`
	Until   string   `yaml:"Until"`
	Ramp    string   `yaml:"Ramp"`
	Plan    string   `yaml:"Plan"`
	SaveTo  []string `yaml:"SaveTo"`

	// Infrastructure
	Provider         string   `yaml:"Provider"`
	Regions          []string `yaml:"Regions"`
	InstanceType     string   `yaml:"InstanceType"`
	ProvisionTimeout string   `yaml:"ProvisionTimeout"`
	TerminateTimeout string   `yaml:"TerminateTimeout"`
	Yes              bool     `yaml:"Yes"`
	DryRun           bool     `yaml:"DryRun"`

	// Remote instance
	RemoteUser string `yaml:"RemoteUser"`

	// AMI
	AMIOS     string `yaml:"AMIOS"`
	AMIOwners string `yaml:"AMIOwners"`

	// Go and lemmings
	GoVersion       string `yaml:"GoVersion"`
	LemmingsPackage string `yaml:"LemmingsPackage"`

	// EBS volumes
	RootVolumeSize int    `yaml:"RootVolumeSize"`
	RootVolumeType string `yaml:"RootVolumeType"`
	TempVolumeSize int    `yaml:"TempVolumeSize"`
	TempVolumeType string `yaml:"TempVolumeType"`
}

// validVolumeTypes is the set of accepted EBS volume type strings.
var validVolumeTypes = map[string]struct{}{
	"gp2": {},
	"gp3": {},
	"io2": {},
}

// loadConfigFile reads a sovereign YAML config file from path and
// merges it into cfg. Values already set via flags take precedence
// over values in the config file.
//
// Returns nil if path is empty. Returns an error if the file exists
// but cannot be read or parsed.
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

// mergeConfigFile applies values from file into cfg only when the
// corresponding cfg field is still at its zero value. This ensures
// that any value explicitly set by a flag (even if it matches a
// default) is never overwritten by the config file.
//
// Exception: bool flags (Yes, DryRun) and numeric flags (Planets,
// Terrain, Pack, RootVolumeSize, TempVolumeSize) use zero-value
// checks directly since false/0 unambiguously means "not set via flag"
// in this context. Duration fields compare against the compile-time
// defaults because duration zero means "not set".
func mergeConfigFile(file *SovereignConfigFile, cfg *SovereignConfig) error {
	// String fields: only merge when cfg field is empty string.
	// This means any flag-set value, including one that matches a
	// default, is preserved.
	if cfg.Hit == "" && file.Hit != "" {
		cfg.Hit = file.Hit
	}
	if cfg.Plan == "" && file.Plan != "" {
		cfg.Plan = file.Plan
	}
	if cfg.Provider == "" && file.Provider != "" {
		cfg.Provider = file.Provider
	}
	if cfg.InstanceType == "" && file.InstanceType != "" {
		cfg.InstanceType = file.InstanceType
	}
	if cfg.RemoteUser == "" && file.RemoteUser != "" {
		cfg.RemoteUser = file.RemoteUser
	}
	if cfg.AMIOS == "" && file.AMIOS != "" {
		cfg.AMIOS = file.AMIOS
	}
	if cfg.AMIOwners == "" && file.AMIOwners != "" {
		cfg.AMIOwners = file.AMIOwners
	}
	if cfg.GoVersion == "" && file.GoVersion != "" {
		cfg.GoVersion = file.GoVersion
	}
	if cfg.LemmingsPackage == "" && file.LemmingsPackage != "" {
		cfg.LemmingsPackage = file.LemmingsPackage
	}
	if cfg.RootVolumeType == "" && file.RootVolumeType != "" {
		cfg.RootVolumeType = file.RootVolumeType
	}
	if cfg.TempVolumeType == "" && file.TempVolumeType != "" {
		cfg.TempVolumeType = file.TempVolumeType
	}

	// Slice fields
	if len(cfg.Regions) == 0 && len(file.Regions) > 0 {
		cfg.Regions = file.Regions
	}
	if len(cfg.SaveTo) == 0 && len(file.SaveTo) > 0 {
		cfg.SaveTo = file.SaveTo
	}

	// Numeric fields: merge when zero.
	if cfg.Planets == 0 && file.Planets > 0 {
		cfg.Planets = file.Planets
	}
	if cfg.Terrain == 0 && file.Terrain > 0 {
		cfg.Terrain = file.Terrain
	}
	if cfg.Pack == 0 && file.Pack > 0 {
		cfg.Pack = file.Pack
	}
	if cfg.RootVolumeSize == 0 && file.RootVolumeSize > 0 {
		cfg.RootVolumeSize = file.RootVolumeSize
	}
	if cfg.TempVolumeSize == 0 && file.TempVolumeSize > 0 {
		cfg.TempVolumeSize = file.TempVolumeSize
	}

	// Bool fields: only set true, never override true with false.
	if !cfg.Yes && file.Yes {
		cfg.Yes = file.Yes
	}
	if !cfg.DryRun && file.DryRun {
		cfg.DryRun = file.DryRun
	}

	// Duration fields: merge when zero.
	if cfg.Until == 0 && file.Until != "" {
		d, err := time.ParseDuration(strings.TrimSpace(file.Until))
		if err != nil {
			return fmt.Errorf("invalid Until duration %q: %w", file.Until, err)
		}
		cfg.Until = d
	}
	if cfg.Ramp == 0 && file.Ramp != "" {
		d, err := time.ParseDuration(strings.TrimSpace(file.Ramp))
		if err != nil {
			return fmt.Errorf("invalid Ramp duration %q: %w", file.Ramp, err)
		}
		cfg.Ramp = d
	}
	if cfg.ProvisionTimeout == 0 && file.ProvisionTimeout != "" {
		d, err := time.ParseDuration(strings.TrimSpace(file.ProvisionTimeout))
		if err != nil {
			return fmt.Errorf("invalid ProvisionTimeout duration %q: %w",
				file.ProvisionTimeout, err)
		}
		cfg.ProvisionTimeout = d
	}
	if cfg.TerminateTimeout == 0 && file.TerminateTimeout != "" {
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
// coherent before NewSovereign is called.
func validateConfig(cfg *SovereignConfig) error {
	// Core
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

	// Infrastructure
	if cfg.ProvisionTimeout <= 0 {
		return fmt.Errorf("sovereign: --provision-timeout must be greater than zero")
	}
	if cfg.TerminateTimeout <= 0 {
		return fmt.Errorf("sovereign: --terminate-timeout must be greater than zero")
	}
	if cfg.InstanceType == "" {
		return fmt.Errorf("sovereign: --instance is required (e.g. t3.medium, c6a.large)")
	}

	// Remote instance
	if cfg.RemoteUser == "" {
		return fmt.Errorf("sovereign: --remote-user must not be empty")
	}

	// AMI
	if cfg.AMIOS == "" {
		return fmt.Errorf("sovereign: --ami-os must not be empty")
	}
	if cfg.AMIOwners == "" {
		return fmt.Errorf("sovereign: --ami-owners must not be empty")
	}

	// Go and lemmings
	if cfg.GoVersion == "" {
		return fmt.Errorf("sovereign: --go-version must not be empty")
	}
	if cfg.LemmingsPackage == "" {
		return fmt.Errorf("sovereign: --lemmings-package must not be empty")
	}

	// EBS volumes
	if cfg.RootVolumeSize <= 0 {
		return fmt.Errorf("sovereign: --root-volume-size must be greater than zero")
	}
	if _, ok := validVolumeTypes[cfg.RootVolumeType]; !ok {
		return fmt.Errorf("sovereign: --root-volume-type must be one of gp2, gp3, io2 -- got %q",
			cfg.RootVolumeType)
	}
	if cfg.TempVolumeSize <= 0 {
		return fmt.Errorf("sovereign: --temp-volume-size must be greater than zero")
	}
	if _, ok := validVolumeTypes[cfg.TempVolumeType]; !ok {
		return fmt.Errorf("sovereign: --temp-volume-type must be one of gp2, gp3, io2 -- got %q",
			cfg.TempVolumeType)
	}

	return nil
}

// exampleConfigFile returns a complete example sovereign.yml.
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

Provider: aws
Regions:
  - us-east-1
  - us-west-1
  - eu-west-1
  - ap-southeast-1
  - sa-east-1

InstanceType: c6a.large

RemoteUser: rocky
AMIOS: "Rocky-9-EC2-Base-9.*-x86_64*"
AMIOwners: "792107900819"

GoVersion: "1.25.0"
LemmingsPackage: "github.com/andreimerlescu/lemmings@latest"

RootVolumeSize: 30
RootVolumeType: gp2
TempVolumeSize: 10
TempVolumeType: gp2

ProvisionTimeout: 5m
TerminateTimeout: 5m

SaveTo:
  - .
  - s3://my-bucket/sovereign-reports
`
}
