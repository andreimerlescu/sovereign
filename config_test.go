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
RemoteUser: rocky
AMIOS: "Rocky-9-EC2-Base-9.*-x86_64*"
AMIOwners: "792107900819"
GoVersion: "1.25.0"
LemmingsPackage: "github.com/andreimerlescu/lemmings@latest"
RootVolumeSize: 30
RootVolumeType: gp2
TempVolumeSize: 10
TempVolumeType: gp2
`
	err := os.WriteFile(path, []byte(contents), 0644)
	requireNoError(t, err, "write test config file")

	cfg := testConfig()
	cfg.Hit = defaultHit
	cfg.Planets = defaultPlanets
	cfg.Terrain = defaultTerrain
	cfg.Pack = defaultPack
	cfg.Until = defaultUntil
	cfg.Ramp = defaultRamp
	cfg.Regions = []string{}
	cfg.InstanceType = ""
	cfg.SaveTo = []string{defaultSaveTo}
	cfg.RemoteUser = defaultRemoteUser
	cfg.AMIOS = defaultAMIOS
	cfg.AMIOwners = defaultAMIOwners
	cfg.GoVersion = defaultGoVersion
	cfg.LemmingsPackage = defaultLemmingsPackage
	cfg.RootVolumeSize = defaultRootVolumeSize
	cfg.RootVolumeType = defaultRootVolumeType
	cfg.TempVolumeSize = defaultTempVolumeSize
	cfg.TempVolumeType = defaultTempVolumeType

	err = loadConfigFile(path, &cfg)
	requireNoError(t, err, "loadConfigFile valid file")

	requireEqual(t, cfg.Hit, "https://example.com", "Hit from config file")
	requireEqual(t, cfg.Planets, 3, "Planets from config file")
	requireEqual(t, cfg.Terrain, int64(10), "Terrain from config file")
	requireEqual(t, cfg.Pack, int64(10), "Pack from config file")
	requireEqual(t, cfg.Until, 60*time.Second, "Until from config file")
	requireEqual(t, cfg.Ramp, 2*time.Minute, "Ramp from config file")
	requireEqual(t, cfg.InstanceType, "t3.large", "InstanceType from config file")
	requireEqual(t, len(cfg.Regions), 2, "Regions count from config file")
	requireEqual(t, cfg.Regions[0], "us-east-1", "First region from config file")
	requireEqual(t, cfg.Yes, true, "Yes from config file")
	requireEqual(t, cfg.RemoteUser, "rocky", "RemoteUser from config file")
	requireEqual(t, cfg.AMIOS, "Rocky-9-EC2-Base-9.*-x86_64*", "AMIOS from config file")
	requireEqual(t, cfg.AMIOwners, "792107900819", "AMIOwners from config file")
	requireEqual(t, cfg.GoVersion, "1.25.0", "GoVersion from config file")
	requireEqual(t, cfg.LemmingsPackage, "github.com/andreimerlescu/lemmings@latest",
		"LemmingsPackage from config file")
	requireEqual(t, cfg.RootVolumeSize, 30, "RootVolumeSize from config file")
	requireEqual(t, cfg.RootVolumeType, "gp2", "RootVolumeType from config file")
	requireEqual(t, cfg.TempVolumeSize, 10, "TempVolumeSize from config file")
	requireEqual(t, cfg.TempVolumeType, "gp2", "TempVolumeType from config file")
}

// TestLoadConfigFile_EmptyPath verifies that an empty path is a no-op.
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

// TestLoadConfigFile_MalformedYAML verifies that invalid YAML returns
// a non-nil error.
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
func TestLoadConfigFile_FlagsOverrideConfigFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sovereign.yml")
	contents := `
Hit: https://from-config-file.com
Planets: 99
RemoteUser: ubuntu
`
	err := os.WriteFile(path, []byte(contents), 0644)
	requireNoError(t, err, "write config file")

	cfg := testConfig()
	cfg.Hit = "https://from-flag.com"
	cfg.Planets = 3
	cfg.RemoteUser = "rocky"

	err = loadConfigFile(path, &cfg)
	requireNoError(t, err, "loadConfigFile flags override")

	requireEqual(t, cfg.Hit, "https://from-flag.com",
		"Hit flag value must not be overwritten by config file")
	requireEqual(t, cfg.Planets, 3,
		"Planets flag value must not be overwritten by config file")
	requireEqual(t, cfg.RemoteUser, "rocky",
		"RemoteUser flag value must not be overwritten by config file")
}

// -- mergeConfigFile ---------------------------------------------------------

// TestMergeConfigFile_AllDefaults verifies that all fields from the
// config file are applied when the config holds default values.
func TestMergeConfigFile_AllDefaults(t *testing.T) {
	file := &SovereignConfigFile{
		Hit:             "https://merged.com",
		Planets:         7,
		Terrain:         20,
		Pack:            20,
		Until:           "45s",
		Ramp:            "3m",
		Plan:            "merged.plan.yml",
		Regions:         []string{"eu-west-1"},
		InstanceType:    "c6a.large",
		SaveTo:          []string{"/tmp/merged"},
		ProvisionTimeout: "8m",
		TerminateTimeout: "8m",
		Yes:             true,
		DryRun:          true,
		RemoteUser:      "ubuntu",
		AMIOS:           "ubuntu/images/hvm-ssd/ubuntu-*-amd64*",
		AMIOwners:       "amazon",
		GoVersion:       "1.24.0",
		LemmingsPackage: "github.com/andreimerlescu/lemmings@v1.0.0",
		RootVolumeSize:  60,
		RootVolumeType:  "gp3",
		TempVolumeSize:  20,
		TempVolumeType:  "gp3",
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
		Provider:         defaultProvider,
		RemoteUser:       defaultRemoteUser,
		AMIOS:            defaultAMIOS,
		AMIOwners:        defaultAMIOwners,
		GoVersion:        defaultGoVersion,
		LemmingsPackage:  defaultLemmingsPackage,
		RootVolumeSize:   defaultRootVolumeSize,
		RootVolumeType:   defaultRootVolumeType,
		TempVolumeSize:   defaultTempVolumeSize,
		TempVolumeType:   defaultTempVolumeType,
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
	requireEqual(t, cfg.RemoteUser, "ubuntu", "merged RemoteUser")
	requireEqual(t, cfg.AMIOS, "ubuntu/images/hvm-ssd/ubuntu-*-amd64*", "merged AMIOS")
	requireEqual(t, cfg.AMIOwners, "amazon", "merged AMIOwners")
	requireEqual(t, cfg.GoVersion, "1.24.0", "merged GoVersion")
	requireEqual(t, cfg.LemmingsPackage, "github.com/andreimerlescu/lemmings@v1.0.0",
		"merged LemmingsPackage")
	requireEqual(t, cfg.RootVolumeSize, 60, "merged RootVolumeSize")
	requireEqual(t, cfg.RootVolumeType, "gp3", "merged RootVolumeType")
	requireEqual(t, cfg.TempVolumeSize, 20, "merged TempVolumeSize")
	requireEqual(t, cfg.TempVolumeType, "gp3", "merged TempVolumeType")
}

// TestMergeConfigFile_DoesNotOverrideNonDefaults verifies that
// non-default config values are preserved.
func TestMergeConfigFile_DoesNotOverrideNonDefaults(t *testing.T) {
	file := &SovereignConfigFile{
		Hit:            "https://from-file.com",
		Planets:        99,
		RemoteUser:     "ubuntu",
		RootVolumeSize: 999,
		RootVolumeType: "io2",
	}

	cfg := SovereignConfig{
		Hit:            "https://from-flag.com",
		Planets:        5,
		Terrain:        int64(25),
		Pack:           int64(25),
		Until:          defaultUntil,
		Ramp:           defaultRamp,
		SaveTo:         []string{defaultSaveTo},
		RemoteUser:     "rocky",
		RootVolumeSize: 30,
		RootVolumeType: "gp2",
	}

	err := mergeConfigFile(file, &cfg)
	requireNoError(t, err, "mergeConfigFile preserves non-defaults")

	requireEqual(t, cfg.Hit, "https://from-flag.com", "Hit must not be overridden")
	requireEqual(t, cfg.Planets, 5, "Planets must not be overridden")
	requireEqual(t, cfg.RemoteUser, "rocky", "RemoteUser must not be overridden")
	requireEqual(t, cfg.RootVolumeSize, 30, "RootVolumeSize must not be overridden")
	requireEqual(t, cfg.RootVolumeType, "gp2", "RootVolumeType must not be overridden")
}

// TestMergeConfigFile_InvalidUntilDuration verifies that a malformed
// Until duration string returns a non-nil error.
func TestMergeConfigFile_InvalidUntilDuration(t *testing.T) {
	file := &SovereignConfigFile{Until: "not-a-duration"}
	cfg := SovereignConfig{Until: defaultUntil, Ramp: defaultRamp}
	err := mergeConfigFile(file, &cfg)
	requireError(t, err, "mergeConfigFile invalid Until duration")
}

// TestMergeConfigFile_InvalidRampDuration verifies that a malformed
// Ramp duration string returns a non-nil error.
func TestMergeConfigFile_InvalidRampDuration(t *testing.T) {
	file := &SovereignConfigFile{Ramp: "not-a-duration"}
	cfg := SovereignConfig{Until: defaultUntil, Ramp: defaultRamp}
	err := mergeConfigFile(file, &cfg)
	requireError(t, err, "mergeConfigFile invalid Ramp duration")
}

// TestMergeConfigFile_InvalidProvisionTimeout verifies that a malformed
// ProvisionTimeout duration string returns a non-nil error.
func TestMergeConfigFile_InvalidProvisionTimeout(t *testing.T) {
	file := &SovereignConfigFile{ProvisionTimeout: "not-a-duration"}
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
	file := &SovereignConfigFile{TerminateTimeout: "not-a-duration"}
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
	requireEqual(t, cfg.Hit, original, "Hit unchanged for empty config file")
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

// TestValidateConfig_EmptyHit verifies that an empty Hit returns error.
func TestValidateConfig_EmptyHit(t *testing.T) {
	cfg := testConfig()
	cfg.Hit = ""
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig empty Hit")
}

// TestValidateConfig_InvalidHitScheme verifies that a Hit value
// not beginning with http returns a non-nil error.
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

// TestValidateConfig_ZeroUntil verifies that Until of zero returns error.
func TestValidateConfig_ZeroUntil(t *testing.T) {
	cfg := testConfig()
	cfg.Until = 0
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig zero Until")
}

// TestValidateConfig_ZeroRamp verifies that Ramp of zero returns error.
func TestValidateConfig_ZeroRamp(t *testing.T) {
	cfg := testConfig()
	cfg.Ramp = 0
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig zero Ramp")
}

// TestValidateConfig_ZeroProvisionTimeout verifies that a zero
// ProvisionTimeout returns a non-nil error.
func TestValidateConfig_ZeroProvisionTimeout(t *testing.T) {
	cfg := testConfig()
	cfg.ProvisionTimeout = 0
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig zero ProvisionTimeout")
}

// TestValidateConfig_ZeroTerminateTimeout verifies that a zero
// TerminateTimeout returns a non-nil error.
func TestValidateConfig_ZeroTerminateTimeout(t *testing.T) {
	cfg := testConfig()
	cfg.TerminateTimeout = 0
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig zero TerminateTimeout")
}

// TestValidateConfig_MissingPlanFile verifies that a Plan path that
// does not exist returns a non-nil error.
func TestValidateConfig_MissingPlanFile(t *testing.T) {
	cfg := testConfig()
	cfg.Plan = "/nonexistent/path/lemmings.plan.yml"
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig missing plan file")
}

// TestValidateConfig_ValidPlanFile verifies that an existing Plan
// path passes validation.
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

// TestValidateConfig_HTTPSHitValid verifies that an HTTPS Hit passes.
func TestValidateConfig_HTTPSHitValid(t *testing.T) {
	cfg := testConfig()
	cfg.Hit = "https://example.com/"
	err := validateConfig(&cfg)
	requireNoError(t, err, "validateConfig HTTPS Hit valid")
}

// TestValidateConfig_EmptyInstanceType verifies that an empty
// InstanceType returns a non-nil error.
func TestValidateConfig_EmptyInstanceType(t *testing.T) {
	cfg := testConfig()
	cfg.InstanceType = ""
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig empty InstanceType")
}

// TestValidateConfig_EmptyRemoteUser verifies that an empty
// RemoteUser returns a non-nil error.
func TestValidateConfig_EmptyRemoteUser(t *testing.T) {
	cfg := testConfig()
	cfg.RemoteUser = ""
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig empty RemoteUser")
}

// TestValidateConfig_EmptyAMIOS verifies that an empty AMIOS
// returns a non-nil error.
func TestValidateConfig_EmptyAMIOS(t *testing.T) {
	cfg := testConfig()
	cfg.AMIOS = ""
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig empty AMIOS")
}

// TestValidateConfig_EmptyAMIOwners verifies that empty AMIOwners
// returns a non-nil error.
func TestValidateConfig_EmptyAMIOwners(t *testing.T) {
	cfg := testConfig()
	cfg.AMIOwners = ""
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig empty AMIOwners")
}

// TestValidateConfig_EmptyGoVersion verifies that an empty GoVersion
// returns a non-nil error.
func TestValidateConfig_EmptyGoVersion(t *testing.T) {
	cfg := testConfig()
	cfg.GoVersion = ""
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig empty GoVersion")
}

// TestValidateConfig_EmptyLemmingsPackage verifies that an empty
// LemmingsPackage returns a non-nil error.
func TestValidateConfig_EmptyLemmingsPackage(t *testing.T) {
	cfg := testConfig()
	cfg.LemmingsPackage = ""
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig empty LemmingsPackage")
}

// TestValidateConfig_ZeroRootVolumeSize verifies that a zero
// RootVolumeSize returns a non-nil error.
func TestValidateConfig_ZeroRootVolumeSize(t *testing.T) {
	cfg := testConfig()
	cfg.RootVolumeSize = 0
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig zero RootVolumeSize")
}

// TestValidateConfig_InvalidRootVolumeType verifies that an invalid
// RootVolumeType returns a non-nil error.
func TestValidateConfig_InvalidRootVolumeType(t *testing.T) {
	cfg := testConfig()
	cfg.RootVolumeType = "invalid"
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig invalid RootVolumeType")
}

// TestValidateConfig_ZeroTempVolumeSize verifies that a zero
// TempVolumeSize returns a non-nil error.
func TestValidateConfig_ZeroTempVolumeSize(t *testing.T) {
	cfg := testConfig()
	cfg.TempVolumeSize = 0
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig zero TempVolumeSize")
}

// TestValidateConfig_InvalidTempVolumeType verifies that an invalid
// TempVolumeType returns a non-nil error.
func TestValidateConfig_InvalidTempVolumeType(t *testing.T) {
	cfg := testConfig()
	cfg.TempVolumeType = "magnetic"
	err := validateConfig(&cfg)
	requireError(t, err, "validateConfig invalid TempVolumeType")
}

// TestValidateConfig_ValidVolumeTypes verifies that all accepted
// volume types pass validation.
func TestValidateConfig_ValidVolumeTypes(t *testing.T) {
	for _, vt := range []string{"gp2", "gp3", "io2"} {
		cfg := testConfig()
		cfg.RootVolumeType = vt
		cfg.TempVolumeType = vt
		err := validateConfig(&cfg)
		requireNoError(t, err, "validateConfig valid volume type "+vt)
	}
}

// -- exampleConfigFile -------------------------------------------------------

// TestExampleConfigFile_NonEmpty verifies that exampleConfigFile
// returns a non-empty string.
func TestExampleConfigFile_NonEmpty(t *testing.T) {
	got := exampleConfigFile()
	requireTrue(t, len(got) > 0, "exampleConfigFile non-empty")
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
		Provider:         defaultProvider,
		RemoteUser:       defaultRemoteUser,
		AMIOS:            defaultAMIOS,
		AMIOwners:        defaultAMIOwners,
		GoVersion:        defaultGoVersion,
		LemmingsPackage:  defaultLemmingsPackage,
		RootVolumeSize:   defaultRootVolumeSize,
		RootVolumeType:   defaultRootVolumeType,
		TempVolumeSize:   defaultTempVolumeSize,
		TempVolumeType:   defaultTempVolumeType,
	}
	err = loadConfigFile(path, &cfg)
	requireNoError(t, err, "exampleConfigFile parses as valid YAML")
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkValidateConfig measures validateConfig throughput.
func BenchmarkValidateConfig(b *testing.B) {
	cfg := testConfig()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validateConfig(&cfg)
	}
}

// BenchmarkMergeConfigFile measures mergeConfigFile throughput.
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
		RemoteUser:       "ubuntu",
		AMIOS:            "ubuntu/images/hvm-ssd/ubuntu-*-amd64*",
		AMIOwners:        "amazon",
		GoVersion:        "1.24.0",
		LemmingsPackage:  "github.com/andreimerlescu/lemmings@latest",
		RootVolumeSize:   60,
		RootVolumeType:   "gp3",
		TempVolumeSize:   20,
		TempVolumeType:   "gp3",
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
			Provider:         defaultProvider,
			RemoteUser:       defaultRemoteUser,
			AMIOS:            defaultAMIOS,
			AMIOwners:        defaultAMIOwners,
			GoVersion:        defaultGoVersion,
			LemmingsPackage:  defaultLemmingsPackage,
			RootVolumeSize:   defaultRootVolumeSize,
			RootVolumeType:   defaultRootVolumeType,
			TempVolumeSize:   defaultTempVolumeSize,
			TempVolumeType:   defaultTempVolumeType,
		}
		mergeConfigFile(file, &cfg)
	}
}

// -- Fuzz --------------------------------------------------------------------

// FuzzMergeConfigFile verifies that mergeConfigFile never panics on
// arbitrary duration string inputs.
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
		_ = mergeConfigFile(file, &cfg)
	})
}

// FuzzDomainFromURL verifies that domainFromURL never panics and
// always returns a non-empty string for non-empty input.
func FuzzDomainFromURL(f *testing.F) {
	f.Add("https://example.com")
	f.Add("http://localhost:8080/")
	f.Add("https://myapp.com/path/to/resource")
	f.Add("")

	f.Fuzz(func(t *testing.T, url string) {
		got := domainFromURL(url)
		if len(url) > 0 && len(got) == 0 {
			t.Errorf("domainFromURL(%q) returned empty string for non-empty input", url)
		}
	})
}

// FuzzFormatBytes verifies that formatBytes never panics and always
// returns a non-empty string for non-negative input.
func FuzzFormatBytes(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(1024))
	f.Add(int64(1048576))
	f.Add(int64(1073741824))

	f.Fuzz(func(t *testing.T, b int64) {
		if b < 0 {
			return
		}
		got := formatBytes(b)
		if len(got) == 0 {
			t.Errorf("formatBytes(%d) returned empty string", b)
		}
	})
}
