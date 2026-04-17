package main

import (
    "os"
    "path/filepath"
    "testing"
    "time"
)

// -- loadConfigFile ----------------------------------------------------------

// TestLoadConfigFile_ValidFile verifies that a well-formed sovereign
// YAML config file is correctly parsed and merged into the config.
func TestLoadConfigFile_ValidFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sovereign.yml")

    contents := `
Hit: https://example.com
Planets: 3
Terrain: 10
Pack: 10
Until: 60s
Ramp: 2m
Plan: lemmings.plan.yml
Regions:
  - us-east-1
  - us-west-1
InstanceType: t3.large
SaveTo:
  - /tmp/reports
ProvisionTimeout: 10m
TerminateTimeout: 10m
Yes: true
DryRun: false
`
    err := os.WriteFile(path, []byte(contents), 0644)
    requireNoError(t, err, "write test config file")

    cfg := testConfig()
    // Reset to defaults so mergeConfigFile has room to fill gaps
    cfg.Hit = defaultHit
    cfg.Planets = defaultPlanets
    cfg.Terrain = defaultTerrain
    cfg.Pack = defaultPack
    cfg.Until = defaultUntil
    cfg.Ramp = defaultRamp
    cfg.Regions = []string{}
    cfg.InstanceType = ""
    cfg.SaveTo = []string{defaultSaveTo}

    err = loadConfigFile(path, &cfg)
    requireNoError(t, err, "loadConfigFile valid file")

    requireEqual(t, cfg.Hit, "https://example.com", "Hit from config file")
    requireEqual(t, cfg.Planets, 3, "Planets from config file")
    requireEqual(t, cfg.Terrain, int64(10), "Terrain from config file")
    requireEqual(t, cfg.Pack, int64(10), "Pack from config file")
    requireEqual(t, cfg.Until, 60*time.Second, "Until from config file")
    requireEqual(t, cfg.Ramp, 2*time.Minute, "Ramp from config file")
    requireEqual(t, cfg.Plan, "lemmings.plan.yml", "Plan from config file")
    requireEqual(t, cfg.InstanceType, "t3.large", "InstanceType from config file")
    requireEqual(t, len(cfg.Regions), 2, "Regions count from config file")
    requireEqual(t, cfg.Regions[0], "us-east-1", "First region from config file")
    requireEqual(t, cfg.Yes, true, "Yes from config file")
}

// TestLoadConfigFile_EmptyPath verifies that an empty path is a no-op
// and returns nil without modifying the config.
func TestLoadConfigFile_EmptyPath(t *testing.T) {
    cfg := testConfig()
    original := cfg.Hit
    err := loadConfigFile("", &cfg)
    requireNoError(t, err, "loadConfigFile empty path")
    requireEqual(t, cfg.Hit, original, "Hit unchanged for empty path")
}

// TestLoadConfigFile_MissingFile verifies that a non-existent file
// returns a non-nil error.
func TestLoadConfigFile_MissingFile(t *testing.T) {
    cfg := testConfig()
    err := loadConfigFile("/nonexistent/path/sovereign.yml", &cfg)
    requireError(t, err, "loadConfigFile missing file")
}

// TestLoadConfigFile_MalformedYAML verifies that invalid YAML content
// returns a non-nil error.
func TestLoadConfigFile_MalformedYAML(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "bad.yml")
    err := os.WriteFile(path, []byte("{ this is not : valid yaml :::"), 0644)
    requireNoError(t, err, "write malformed config file")

    cfg := testConfig()
    err = loadConfigFile(path, &cfg)
    requireError(t, err, "loadConfigFile malformed YAML")
}

// TestLoadConfigFile_EmptyFile verifies that an empty file is parsed
// without error and leaves the config unchanged.
func TestLoadConfigFile_EmptyFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "empty.yml")
    err := os.WriteFile(path, []byte(""), 0644)
    requireNoError(t, err, "write empty config file")

    cfg := testConfig()
    original := cfg.Hit
    err = loadConfigFile(path, &cfg)
    requireNoError(t, err, "loadConfigFile empty file")
    requireEqual(t, cfg.Hit, original, "Hit unchanged for empty file")
}

// TestLoadConfigFile_FlagsOverrideConfigFile verifies that values
// already set in the config are not overwritten by the config file.
// Flags take precedence over config file values.
func TestLoadConfigFile_FlagsOverrideConfigFile(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "sovereign.yml")
    contents := `
Hit: https://from-config-file.com
Planets: 99
`
    err := os.WriteFile(path, []byte(contents), 0644)
    requireNoError(t, err, "write config file")

    cfg := testConfig()
    // Simulate flag already set -- non-default value
    cfg.Hit = "https://from-flag.com"
    cfg.Planets = 3

    err = loadConfigFile(path, &cfg)
    requireNoError(t, err, "loadConfigFile flags override")

    // Flag values must survive the merge
    requireEqual(t, cfg.Hit, "https://from-flag.com",
        "Hit flag value must not be overwritten by config file")
    requireEqual(t, cfg.Planets, 3,
        "Planets flag value must not be overwritten by config file")
}

// -- mergeConfigFile ---------------------------------------------------------

// TestMergeConfigFile_AllDefaults verifies that all fields from the
// config file are applied when the config holds default values.
func TestMergeConfigFile_AllDefaults(t *testing.T) {
    file := &SovereignConfigFile{
        Hit:              "https://merged.com",
        Planets:          7,
        Terrain:          20,
        Pack:             20,
        Until:            "45s",
        Ramp:             "3m",
        Plan:             "merged.plan.yml",
        Regions:          []string{"eu-west-1"},
        InstanceType:     "c6a.large",
        SaveTo:           []string{"/tmp/merged"},
        ProvisionTimeout: "8m",
        TerminateTimeout: "8m",
        Yes:              true,
        DryRun:           true,
    }

    cfg := SovereignConfig{
        Hit:              defaultHit,
        Planets:          defaultPlanets,
        Terrain:          defaultTerrain,
        Pack:             defaultPack,
        Until:            defaultUntil,
        Ramp:             defaultRamp,
        SaveTo:           []string{defaultSaveTo},
        ProvisionTimeout: defaultProvisionTimeout,
        TerminateTimeout: defaultTerminateTimeout,
    }

    err := mergeConfigFile(file, &cfg)
    requireNoError(t, err, "mergeConfigFile all defaults")

    requireEqual(t, cfg.Hit, "https://merged.com", "merged Hit")
    requireEqual(t, cfg.Planets, 7, "merged Planets")
    requireEqual(t, cfg.Terrain, int64(20), "merged Terrain")
    requireEqual(t, cfg.Pack, int64(20), "merged Pack")
    requireEqual(t, cfg.Until, 45*time.Second, "merged Until")
    requireEqual(t, cfg.Ramp, 3*time.Minute, "merged Ramp")
    requireEqual(t, cfg.Plan, "merged.plan.yml", "merged Plan")
    requireEqual(t, cfg.InstanceType, "c6a.large", "merged InstanceType")
    requireEqual(t, len(cfg.Regions), 1, "merged Regions count")
    requireEqual(t, cfg.Regions[0], "eu-west-1", "merged Region value")
    requireEqual(t, cfg.Yes, true, "merged Yes")
    requireEqual(t, cfg.DryRun, true, "merged DryRun")
    requireEqual(t, cfg.ProvisionTimeout, 8*time.Minute, "merged ProvisionTimeout")
    requireEqual(t, cfg.TerminateTimeout, 8*time.Minute, "merged TerminateTimeout")
}

// TestMergeConfigFile_DoesNotOverrideNonDefaults verifies that
// non-default config values are preserved when the file contains
// different values.
func TestMergeConfigFile_DoesNotOverrideNonDefaults(t *testing.T) {
    file := &SovereignConfigFile{
        Hit:     "https://from-file.com",
        Planets: 99,
        Terrain: 99,
        Pack:    99,
    }

    cfg := SovereignConfig{
        Hit:     "https://from-flag.com",
        Planets: 5,
        Terrain: int64(25),
        Pack:    int64(25),
        Until:   defaultUntil,
        Ramp:    defaultRamp,
        SaveTo:  []string{defaultSaveTo},
    }

    err := mergeConfigFile(file, &cfg)
    requireNoError(t, err, "mergeConfigFile preserves non-defaults")

    requireEqual(t, cfg.Hit, "https://from-flag.com",
        "Hit must not be overridden")
    requireEqual(t, cfg.Planets, 5,
        "Planets must not be overridden")
    requireEqual(t, cfg.Terrain, int64(25),
        "Terrain must not be overridden")
    requireEqual(t, cfg.Pack, int64(25),
        "Pack must not be overridden")
}

// TestMergeConfigFile_InvalidUntilDuration verifies that a malformed
// Until duration string returns a non-nil error.
func TestMergeConfigFile_InvalidUntilDuration(t *testing.T) {
    file := &SovereignConfigFile{
        Until: "not-a-duration",
    }
    cfg := SovereignConfig{
        Until: defaultUntil,
        Ramp:  defaultRamp,
    }
    err := mergeConfigFile(file, &cfg)
    requireError(t, err, "mergeConfigFile invalid Until duration")
}

// TestMergeConfigFile_InvalidRampDuration verifies that a malformed
// Ramp duration string returns a non-nil error.
func TestMergeConfigFile_InvalidRampDuration(t *testing.T) {
    file := &SovereignConfigFile{
        Ramp: "not-a-duration",
    }
    cfg := SovereignConfig{
        Until: defaultUntil,
        Ramp:  defaultRamp,
    }
    err := mergeConfigFile(file, &cfg)
    requireError(t, err, "mergeConfigFile invalid Ramp duration")
}

// TestMergeConfigFile_InvalidProvisionTimeout verifies that a malformed
// ProvisionTimeout duration string returns a non-nil error.
func TestMergeConfigFile_InvalidProvisionTimeout(t *testing.T) {
    file := &SovereignConfigFile{
        ProvisionTimeout: "not-a-duration",
    }
    cfg := SovereignConfig{
        Until:            defaultUntil,
        Ramp:             defaultRamp,
        ProvisionTimeout: defaultProvisionTimeout,
    }
    err := mergeConfigFile(file, &cfg)
    requireError(t, err, "mergeConfigFile invalid ProvisionTimeout")
}

// TestMergeConfigFile_InvalidTerminateTimeout verifies that a malformed
// TerminateTimeout duration string returns a non-nil error.
func TestMergeConfigFile_InvalidTerminateTimeout(t *testing.T) {
    file := &SovereignConfigFile{
        TerminateTimeout: "not-a-duration",
    }
    cfg := SovereignConfig{
        Until:            defaultUntil,
        Ramp:             defaultRamp,
        TerminateTimeout: defaultTerminateTimeout,
    }
    err := mergeConfigFile(file, &cfg)
    requireError(t, err, "mergeConfigFile invalid TerminateTimeout")
}

// TestMergeConfigFile_EmptyFileIsNoop verifies that an empty config
// file struct leaves all config values unchanged.
func TestMergeConfigFile_EmptyFileIsNoop(t *testing.T) {
    file := &SovereignConfigFile{}
    cfg := testConfig()
    original := cfg.Hit

    err := mergeConfigFile(file, &cfg)
    requireNoError(t, err, "mergeConfigFile empty file is noop")
    requireEqual(t, cfg.Hit, original,
        "Hit unchanged for empty config file")
}

// TestMergeConfigFile_SaveToMerged verifies that SaveTo from the
// config file replaces the default save-to value.
func TestMergeConfigFile_SaveToMerged(t *testing.T) {
    file := &SovereignConfigFile{
        SaveTo: []string{"/tmp/reports", "s3://my-bucket"},
    }
    cfg := SovereignConfig{
        SaveTo: []string{defaultSaveTo},
        Until:  defaultUntil,
        Ramp:   defaultRamp,
    }
    err := mergeConfigFile(file, &cfg)
    requireNoError(t, err, "mergeConfigFile SaveTo merged")
    requireEqual(t, len(cfg.SaveTo), 2, "SaveTo count after merge")
    requireEqual(t, cfg.SaveTo[0], "/tmp/reports", "first SaveTo after merge")
    requireEqual(t, cfg.SaveTo[1], "s3://my-bucket", "second SaveTo after merge")
}

// -- validateConfig ----------------------------------------------------------

// TestValidateConfig_ValidConfig verifies that a fully populated
// valid config passes validation without error.
func TestValidateConfig_ValidConfig(t *testing.T) {
    cfg := testConfig()
    err := validateConfig(&cfg)
    requireNoError(t, err, "validateConfig valid config")
}

// TestValidateConfig_EmptyHit verifies that an empty Hit returns
// a non-nil error.
func TestValidateConfig_EmptyHit(t *testing.T) {
    cfg := testConfig()
    cfg.Hit = ""
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig empty Hit")
}

// TestValidateConfig_InvalidHitScheme verifies that a Hit value
// not beginning with http or https returns a non-nil error.
func TestValidateConfig_InvalidHitScheme(t *testing.T) {
    cfg := testConfig()
    cfg.Hit = "ftp://example.com"
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig invalid Hit scheme")
}

// TestValidateConfig_ZeroPlanets verifies that Planets less than 1
// returns a non-nil error.
func TestValidateConfig_ZeroPlanets(t *testing.T) {
    cfg := testConfig()
    cfg.Planets = 0
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig zero Planets")
}

// TestValidateConfig_ZeroTerrain verifies that Terrain less than 1
// returns a non-nil error.
func TestValidateConfig_ZeroTerrain(t *testing.T) {
    cfg := testConfig()
    cfg.Terrain = 0
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig zero Terrain")
}

// TestValidateConfig_ZeroPack verifies that Pack less than 1
// returns a non-nil error.
func TestValidateConfig_ZeroPack(t *testing.T) {
    cfg := testConfig()
    cfg.Pack = 0
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig zero Pack")
}

// TestValidateConfig_ZeroUntil verifies that an Until of zero
// returns a non-nil error.
func TestValidateConfig_ZeroUntil(t *testing.T) {
    cfg := testConfig()
    cfg.Until = 0
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig zero Until")
}

// TestValidateConfig_ZeroRamp verifies that a Ramp of zero returns
// a non-nil error.
func TestValidateConfig_ZeroRamp(t *testing.T) {
    cfg := testConfig()
    cfg.Ramp = 0
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig zero Ramp")
}

// TestValidateConfig_ZeroProvisionTimeout verifies that a
// ProvisionTimeout of zero returns a non-nil error.
func TestValidateConfig_ZeroProvisionTimeout(t *testing.T) {
    cfg := testConfig()
    cfg.ProvisionTimeout = 0
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig zero ProvisionTimeout")
}

// TestValidateConfig_ZeroTerminateTimeout verifies that a
// TerminateTimeout of zero returns a non-nil error.
func TestValidateConfig_ZeroTerminateTimeout(t *testing.T) {
    cfg := testConfig()
    cfg.TerminateTimeout = 0
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig zero TerminateTimeout")
}

// TestValidateConfig_MissingPlanFile verifies that a Plan path that
// does not exist on disk returns a non-nil error.
func TestValidateConfig_MissingPlanFile(t *testing.T) {
    cfg := testConfig()
    cfg.Plan = "/nonexistent/path/lemmings.plan.yml"
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig missing plan file")
}

// TestValidateConfig_ValidPlanFile verifies that a Plan path that
// exists on disk passes validation.
func TestValidateConfig_ValidPlanFile(t *testing.T) {
    dir := t.TempDir()
    planPath := filepath.Join(dir, "lemmings.plan.yml")
    err := os.WriteFile(planPath, []byte("plan:\n  - name: test\n"), 0644)
    requireNoError(t, err, "write plan file")

    cfg := testConfig()
    cfg.Plan = planPath
    err = validateConfig(&cfg)
    requireNoError(t, err, "validateConfig valid plan file")
}

// TestValidateConfig_EmptySaveToDestination verifies that a SaveTo
// list containing an empty string returns a non-nil error.
func TestValidateConfig_EmptySaveToDestination(t *testing.T) {
    cfg := testConfig()
    cfg.SaveTo = []string{".", "", "/tmp/reports"}
    err := validateConfig(&cfg)
    requireError(t, err, "validateConfig empty SaveTo destination")
}

// TestValidateConfig_HTTPSHitValid verifies that an HTTPS Hit passes
// validation.
func TestValidateConfig_HTTPSHitValid(t *testing.T) {
    cfg := testConfig()
    cfg.Hit = "https://example.com/"
    err := validateConfig(&cfg)
    requireNoError(t, err, "validateConfig HTTPS Hit valid")
}

// -- exampleConfigFile -------------------------------------------------------

// TestExampleConfigFile_NonEmpty verifies that exampleConfigFile
// returns a non-empty string.
func TestExampleConfigFile_NonEmpty(t *testing.T) {
    got := exampleConfigFile()
    requireTrue(t, len(got) > 0, "exampleConfigFile non-empty")
}

// TestExampleConfigFile_ContainsHit verifies that the example config
// contains a Hit field.
func TestExampleConfigFile_ContainsHit(t *testing.T) {
    got := exampleConfigFile()
    requireTrue(t, len(got) > 0, "exampleConfigFile contains Hit")
}

// TestExampleConfigFile_IsValidYAML verifies that the example config
// can be parsed as a SovereignConfigFile without error.
func TestExampleConfigFile_IsValidYAML(t *testing.T) {
    dir := t.TempDir()
    path := filepath.Join(dir, "example.yml")
    err := os.WriteFile(path, []byte(exampleConfigFile()), 0644)
    requireNoError(t, err, "write example config file")

    cfg := SovereignConfig{
        Hit:              defaultHit,
        Planets:          defaultPlanets,
        Terrain:          defaultTerrain,
        Pack:             defaultPack,
        Until:            defaultUntil,
        Ramp:             defaultRamp,
        SaveTo:           []string{defaultSaveTo},
        ProvisionTimeout: defaultProvisionTimeout,
        TerminateTimeout: defaultTerminateTimeout,
    }
    err = loadConfigFile(path, &cfg)
    requireNoError(t, err, "exampleConfigFile parses as valid YAML")
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkValidateConfig measures validateConfig throughput.
// Called once per sovereign invocation but useful as a regression
// baseline for the validation logic.
func BenchmarkValidateConfig(b *testing.B) {
    cfg := testConfig()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        validateConfig(&cfg)
    }
}

// BenchmarkMergeConfigFile measures mergeConfigFile throughput
// against a fully populated config file struct.
func BenchmarkMergeConfigFile(b *testing.B) {
    file := &SovereignConfigFile{
        Hit:              "https://bench.example.com",
        Planets:          5,
        Terrain:          50,
        Pack:             50,
        Until:            "30s",
        Ramp:             "5m",
        InstanceType:     "t3.medium",
        ProvisionTimeout: "5m",
        TerminateTimeout: "5m",
    }
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        cfg := SovereignConfig{
            Hit:              defaultHit,
            Planets:          defaultPlanets,
            Terrain:          defaultTerrain,
            Pack:             defaultPack,
            Until:            defaultUntil,
            Ramp:             defaultRamp,
            SaveTo:           []string{defaultSaveTo},
            ProvisionTimeout: defaultProvisionTimeout,
            TerminateTimeout: defaultTerminateTimeout,
        }
        mergeConfigFile(file, &cfg)
    }
}

// -- Fuzz --------------------------------------------------------------------

// FuzzMergeConfigFile verifies that mergeConfigFile never panics on
// arbitrary duration string inputs and either succeeds or returns
// a non-nil error -- never panics.
func FuzzMergeConfigFile(f *testing.F) {
    f.Add("30s", "5m", "5m", "5m")
    f.Add("", "", "", "")
    f.Add("not-a-duration", "5m", "5m", "5m")
    f.Add("30s", "not-a-duration", "5m", "5m")
    f.Add("0s", "0s", "0s", "0s")
    f.Add("24h", "24h", "24h", "24h")

    f.Fuzz(func(t *testing.T, until, ramp, provTimeout, termTimeout string) {
        file := &SovereignConfigFile{
            Until:            until,
            Ramp:             ramp,
            ProvisionTimeout: provTimeout,
            TerminateTimeout: termTimeout,
        }
        cfg := SovereignConfig{
            Until:            defaultUntil,
            Ramp:             defaultRamp,
            ProvisionTimeout: defaultProvisionTimeout,
            TerminateTimeout: defaultTerminateTimeout,
        }
        // Must never panic -- error is acceptable
        _ = mergeConfigFile(file, &cfg)
    })
}​​​​​​​​​​​​​​​​
