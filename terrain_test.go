package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform_provider"
)

// -- MockTerraformProvider ---------------------------------------------------

// MockTerraformProvider is a test double for TerraformProvider that
// records calls and returns configurable responses. Used to test
// terrain.go logic without spawning real terraform processes.
type MockTerraformProvider struct {
	name          string
	validateErr   error
	renderErr     error
	initErr       error
	applyErr      error
	destroyErr    error
	outputResult  *tfprovider.ProvisionOutput
	outputErr     error
	statusResult  tfprovider.LifecycleStatus
	statusErr     error
	stateResult   *tfprovider.ProviderState
	stateErr      error
	savedStates   []*tfprovider.ProviderState
	renderedSpecs []tfprovider.HCLSpec
	initCalls     int
	applyCalls    int
	destroyCalls  int
	outputCalls   int
}

func newMockProvider() *MockTerraformProvider {
	return &MockTerraformProvider{
		name:         "mock",
		statusResult: tfprovider.LifecycleUninitialized,
		outputResult: &tfprovider.ProvisionOutput{
			InstanceID:  "i-mockinstance001",
			PublicIPv4:  "54.1.2.3",
			PrivateIPv4: "10.0.1.1",
			Region:      testRegion,
			Zone:        testZone,
		},
	}
}

func (m *MockTerraformProvider) Name() string  { return m.name }
func (m *MockTerraformProvider) Validate() error { return m.validateErr }

func (m *MockTerraformProvider) RenderHCL(spec tfprovider.HCLSpec) error {
	m.renderedSpecs = append(m.renderedSpecs, spec)
	return m.renderErr
}

func (m *MockTerraformProvider) Init(ctx context.Context, spec tfprovider.PlanetSpec) error {
	m.initCalls++
	return m.initErr
}

func (m *MockTerraformProvider) InitOrApply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	m.initCalls++
	m.applyCalls++
	if m.initErr != nil {
		return m.initErr
	}
	return m.applyErr
}

func (m *MockTerraformProvider) Apply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	m.applyCalls++
	return m.applyErr
}

func (m *MockTerraformProvider) Destroy(ctx context.Context, spec tfprovider.PlanetSpec) error {
	m.destroyCalls++
	return m.destroyErr
}

func (m *MockTerraformProvider) Output(ctx context.Context, spec tfprovider.PlanetSpec) (*tfprovider.ProvisionOutput, error) {
	m.outputCalls++
	return m.outputResult, m.outputErr
}

func (m *MockTerraformProvider) Status(ctx context.Context, spec tfprovider.PlanetSpec) (tfprovider.LifecycleStatus, error) {
	return m.statusResult, m.statusErr
}

func (m *MockTerraformProvider) StateBackend(spec tfprovider.PlanetSpec) (string, error) {
	return filepath.Join(spec.WorkDir, "terraform", "terraform.tfstate"), nil
}

func (m *MockTerraformProvider) LoadState(spec tfprovider.PlanetSpec) (*tfprovider.ProviderState, error) {
	return m.stateResult, m.stateErr
}

func (m *MockTerraformProvider) SaveState(spec tfprovider.PlanetSpec, state *tfprovider.ProviderState) error {
	m.savedStates = append(m.savedStates, state)
	return nil
}

func (m *MockTerraformProvider) UnloadState(spec tfprovider.PlanetSpec) error {
	return nil
}

// -- test helper -------------------------------------------------------------

// testSovereignWithMockProvider constructs a Sovereign with a mock
// provider for terrain tests.
func testSovereignWithMockProvider(t *testing.T, mock *MockTerraformProvider) *Sovereign {
	t.Helper()
	s := testSovereign(t)
	s.provider = mock
	return s
}

// -- planetSpec --------------------------------------------------------------

// TestPlanetSpec_FieldsPopulated verifies that planetSpec correctly
// populates PlanetSpec from a planet.
func TestPlanetSpec_FieldsPopulated(t *testing.T) {
	s := testSovereign(t)
	planet := testPlanet(3)
	planet.Region = "eu-west-1"

	spec := s.planetSpec(planet)

	requireEqual(t, spec.PlanetID, 3, "planetSpec PlanetID")
	requireEqual(t, spec.Region, "eu-west-1", "planetSpec Region")
	requireTrue(t, len(spec.WorkDir) > 0, "planetSpec WorkDir non-empty")
	requireTrue(t, containsStr(spec.WorkDir, "planet-3"),
		"planetSpec WorkDir contains planet ID")
}

// TestPlanetSpec_ZoneDerivesFromRegion verifies that the zone is
// derived from the region by appending 'a'.
func TestPlanetSpec_ZoneDerivesFromRegion(t *testing.T) {
	s := testSovereign(t)
	planet := testPlanet(1)
	planet.Region = "us-west-2"

	spec := s.planetSpec(planet)
	requireEqual(t, spec.Zone, "us-west-2a", "planetSpec Zone derived from region")
}

// -- hclSpec -----------------------------------------------------------------

// TestHCLSpec_FieldsPopulated verifies that hclSpec correctly
// populates HCLSpec from a planet and config.
func TestHCLSpec_FieldsPopulated(t *testing.T) {
	t.Setenv("SOVEREIGN_SUBNET_ID", "subnet-test123")
	t.Setenv("SOVEREIGN_SECURITY_GROUP", "sg-test456")

	s := testSovereign(t)
	s.cfg.RemoteUser = "rocky"
	s.cfg.AMIOS = "Rocky-9-EC2-Base-9.*-x86_64*"
	s.cfg.AMIOwners = "792107900819"
	s.cfg.InstanceType = "t3.medium"
	s.cfg.GoVersion = "1.25.0"
	s.cfg.LemmingsPackage = "github.com/andreimerlescu/lemmings@latest"

	planet := testPlanet(2)
	planet.Region = "us-east-1"
	planet.InstanceType = "t3.medium"

	spec := s.hclSpec(planet)

	requireEqual(t, spec.PlanetID, 2, "hclSpec PlanetID")
	requireEqual(t, spec.Region, "us-east-1", "hclSpec Region")
	requireEqual(t, spec.InstanceType, "t3.medium", "hclSpec InstanceType")
	requireEqual(t, spec.RemoteUser, "rocky", "hclSpec RemoteUser")
	requireEqual(t, spec.AMIOS, "Rocky-9-EC2-Base-9.*-x86_64*", "hclSpec AMIOS")
	requireEqual(t, spec.AMIOwners, "792107900819", "hclSpec AMIOwners")
	requireEqual(t, spec.SubnetID, "subnet-test123", "hclSpec SubnetID from env")
	requireEqual(t, spec.SecurityGroupID, "sg-test456",
		"hclSpec SecurityGroupID from env")
	requireEqual(t, spec.GoVersion, "1.25.0", "hclSpec GoVersion")
	requireEqual(t, spec.LemmingsPackage,
		"github.com/andreimerlescu/lemmings@latest", "hclSpec LemmingsPackage")
	requireEqual(t, spec.SaveTo, "/home/rocky/lemmings", "hclSpec SaveTo")
}

// TestHCLSpec_SlugContainsPlanetID verifies that the slug contains
// the planet ID for resource naming.
func TestHCLSpec_SlugContainsPlanetID(t *testing.T) {
	s := testSovereign(t)
	planet := testPlanet(7)
	spec := s.hclSpec(planet)
	requireTrue(t, containsStr(spec.Slug, "7"),
		"hclSpec Slug contains planet ID")
}

// -- markLost ----------------------------------------------------------------

// TestMarkLost_SetsPlanetStatusLost verifies that markLost sets the
// planet status to PlanetStatusLost.
func TestMarkLost_SetsPlanetStatusLost(t *testing.T) {
	s := testSovereignWithMockProvider(t, newMockProvider())
	planet := testPlanet(1)

	err := s.markLost(planet, LedgerEventProvisionFailed,
		fmt.Errorf("simulated failure"))

	requireError(t, err, "markLost returns error")
	requireEqual(t, planet.Status, PlanetStatusLost,
		"markLost sets PlanetStatusLost")
}

// TestMarkLost_AppendsLedgerEvent verifies that markLost appends
// the failure event to the ledger.
func TestMarkLost_AppendsLedgerEvent(t *testing.T) {
	s := testSovereignWithMockProvider(t, newMockProvider())
	planet := testPlanet(1)

	s.markLost(planet, LedgerEventProvisionFailed,
		fmt.Errorf("simulated failure"))

	last := s.ledger.LastEventForPlanet(1)
	requireTrue(t, last != nil, "markLost appends ledger event")
	requireEqual(t, last.EventType, LedgerEventProvisionFailed,
		"markLost appends correct event type")
}

// TestMarkLost_SetsErrorOnPlanet verifies that markLost sets the
// Error field on the planet.
func TestMarkLost_SetsErrorOnPlanet(t *testing.T) {
	s := testSovereignWithMockProvider(t, newMockProvider())
	planet := testPlanet(1)
	simErr := fmt.Errorf("simulated failure")

	s.markLost(planet, LedgerEventProvisionFailed, simErr)

	requireTrue(t, planet.Error != nil, "markLost sets planet.Error")
}

// -- saveProviderState -------------------------------------------------------

// TestSaveProviderState_CallsProviderSaveState verifies that
// saveProviderState calls the provider's SaveState method.
func TestSaveProviderState_CallsProviderSaveState(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	s.saveProviderState(planet, tfprovider.LifecycleApplied, nil)

	requireEqual(t, len(mock.savedStates), 1,
		"saveProviderState calls provider SaveState")
}

// TestSaveProviderState_StatusPreserved verifies that the status
// passed to saveProviderState is written into the saved state.
func TestSaveProviderState_StatusPreserved(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	s.saveProviderState(planet, tfprovider.LifecycleInitialized, nil)

	requireEqual(t, mock.savedStates[0].Status,
		tfprovider.LifecycleInitialized,
		"saveProviderState status preserved")
}

// TestSaveProviderState_OutputPopulatesIPAndID verifies that when a
// ProvisionOutput is provided the IP and instance ID are written.
func TestSaveProviderState_OutputPopulatesIPAndID(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	out := &tfprovider.ProvisionOutput{
		InstanceID:  "i-abc123",
		PublicIPv4:  "1.2.3.4",
		PrivateIPv4: "10.0.0.1",
	}
	s.saveProviderState(planet, tfprovider.LifecycleApplied, out)

	saved := mock.savedStates[0]
	requireEqual(t, saved.InstanceID, "i-abc123",
		"saveProviderState InstanceID from output")
	requireEqual(t, saved.PublicIPv4, "1.2.3.4",
		"saveProviderState PublicIPv4 from output")
}

// -- provisionPlanet ---------------------------------------------------------

// TestProvisionPlanet_Success verifies that a successful provision
// cycle sets the planet to PlanetStatusRunning with IP and instance ID.
func TestProvisionPlanet_Success(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	err := s.provisionPlanet(planet)

	requireNoError(t, err, "provisionPlanet success")
	requireEqual(t, planet.Status, PlanetStatusRunning,
		"provisionPlanet sets running status")
	requireEqual(t, planet.PublicIPv4, "54.1.2.3",
		"provisionPlanet sets PublicIPv4")
	requireEqual(t, planet.InstanceID, "i-mockinstance001",
		"provisionPlanet sets InstanceID")
}

// TestProvisionPlanet_RenderFailureSetsLost verifies that an HCL
// render failure marks the planet lost.
func TestProvisionPlanet_RenderFailureSetsLost(t *testing.T) {
	mock := newMockProvider()
	mock.renderErr = fmt.Errorf("render failed")
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	err := s.provisionPlanet(planet)

	requireError(t, err, "provisionPlanet render failure returns error")
	requireEqual(t, planet.Status, PlanetStatusLost,
		"provisionPlanet render failure sets lost")
}

// TestProvisionPlanet_InitFailureSetsLost verifies that a terraform
// init failure marks the planet lost.
func TestProvisionPlanet_InitFailureSetsLost(t *testing.T) {
	mock := newMockProvider()
	mock.initErr = fmt.Errorf("init failed")
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	err := s.provisionPlanet(planet)

	requireError(t, err, "provisionPlanet init failure returns error")
	requireEqual(t, planet.Status, PlanetStatusLost,
		"provisionPlanet init failure sets lost")
}

// TestProvisionPlanet_ApplyFailureSetsLost verifies that a terraform
// apply failure marks the planet lost.
func TestProvisionPlanet_ApplyFailureSetsLost(t *testing.T) {
	mock := newMockProvider()
	mock.applyErr = fmt.Errorf("apply failed")
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	err := s.provisionPlanet(planet)

	requireError(t, err, "provisionPlanet apply failure returns error")
	requireEqual(t, planet.Status, PlanetStatusLost,
		"provisionPlanet apply failure sets lost")
}

// TestProvisionPlanet_OutputFailureSetsLost verifies that a terraform
// output failure marks the planet lost.
func TestProvisionPlanet_OutputFailureSetsLost(t *testing.T) {
	mock := newMockProvider()
	mock.outputErr = fmt.Errorf("output failed")
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	err := s.provisionPlanet(planet)

	requireError(t, err, "provisionPlanet output failure returns error")
	requireEqual(t, planet.Status, PlanetStatusLost,
		"provisionPlanet output failure sets lost")
}

// TestProvisionPlanet_LedgerRecordsProvisionComplete verifies that
// a successful provision records a complete event in the ledger.
func TestProvisionPlanet_LedgerRecordsProvisionComplete(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	s.provisionPlanet(planet)

	requireTrue(t, s.ledger.PlanetProvisioned(1),
		"provisionPlanet ledger records provision complete")
}

// -- destroyPlanet -----------------------------------------------------------

// TestDestroyPlanet_Success verifies that a successful destroy
// completes without error.
func TestDestroyPlanet_Success(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testProvisionedPlanet(1)

	err := s.destroyPlanet(planet)

	requireNoError(t, err, "destroyPlanet success")
	requireEqual(t, mock.destroyCalls, 1,
		"destroyPlanet calls provider Destroy once")
}

// TestDestroyPlanet_FailureReturnsError verifies that a provider
// destroy failure returns a non-nil error.
func TestDestroyPlanet_FailureReturnsError(t *testing.T) {
	mock := newMockProvider()
	mock.destroyErr = fmt.Errorf("destroy failed")
	s := testSovereignWithMockProvider(t, mock)
	planet := testProvisionedPlanet(1)

	err := s.destroyPlanet(planet)

	requireError(t, err, "destroyPlanet failure returns error")
}

// TestDestroyPlanet_LedgerRecordsDestroyComplete verifies that a
// successful destroy records a complete event in the ledger.
func TestDestroyPlanet_LedgerRecordsDestroyComplete(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testProvisionedPlanet(1)

	s.destroyPlanet(planet)

	last := s.ledger.LastEventForPlanet(1)
	requireTrue(t, last != nil, "destroyPlanet ledger event exists")
	requireEqual(t, last.EventType, LedgerEventDestroyComplete,
		"destroyPlanet ledger records destroy complete")
}

// -- planetWorkDir -----------------------------------------------------------

// TestPlanetWorkDir_ContainsPlanetID verifies that the work directory
// path includes the planet ID.
func TestPlanetWorkDir_ContainsPlanetID(t *testing.T) {
	s := testSovereign(t)
	planet := testPlanet(5)
	dir := s.planetWorkDir(planet)
	requireTrue(t, containsStr(dir, "planet-5"),
		"planetWorkDir contains planet ID")
	requireTrue(t, containsStr(dir, ".sovereignd"),
		"planetWorkDir contains sovereignd")
}

// -- provisionTerrain --------------------------------------------------------

// TestProvisionTerrain_SinglePlanet verifies that a single planet
// is provisioned successfully.
func TestProvisionTerrain_SinglePlanet(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	s.cfg.Planets = 1
	s.planets = s.initPlanets()

	err := s.provisionTerrain()

	requireNoError(t, err, "provisionTerrain single planet")
	requireEqual(t, s.planets[0].Status, PlanetStatusRunning,
		"provisionTerrain single planet running")
}

// TestProvisionTerrain_EmptyPlanetsReturnsError verifies that an
// empty planet slice returns an error.
func TestProvisionTerrain_EmptyPlanetsReturnsError(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	s.planets = []*Planet{}

	err := s.provisionTerrain()

	requireError(t, err, "provisionTerrain empty planets")
}

// TestProvisionTerrain_FirstPlanetFailureReturnsError verifies that
// a first planet failure returns an error immediately.
func TestProvisionTerrain_FirstPlanetFailureReturnsError(t *testing.T) {
	mock := newMockProvider()
	mock.initErr = fmt.Errorf("init failed")
	s := testSovereignWithMockProvider(t, mock)
	s.cfg.Planets = 3
	s.planets = s.initPlanets()

	err := s.provisionTerrain()

	requireError(t, err, "provisionTerrain first planet failure")
}

// TestProvisionTerrain_MultiplePlanets verifies that multiple planets
// are all provisioned when the provider succeeds.
func TestProvisionTerrain_MultiplePlanets(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	s.cfg.Planets = 4
	s.cfg.Regions = []string{testRegion}
	s.planets = s.initPlanets()

	err := s.provisionTerrain()

	requireNoError(t, err, "provisionTerrain multiple planets")
	for _, p := range s.planets {
		requireEqual(t, p.Status, PlanetStatusRunning,
			fmt.Sprintf("provisionTerrain planet %d running", p.ID))
	}
}

// -- destroyTerrain ----------------------------------------------------------

// TestDestroyTerrain_AllPlanets verifies that all provisioned planets
// are destroyed.
func TestDestroyTerrain_AllPlanets(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	s.planets = []*Planet{
		testProvisionedPlanet(1),
		testProvisionedPlanet(2),
		testProvisionedPlanet(3),
	}

	err := s.destroyTerrain()

	requireNoError(t, err, "destroyTerrain all planets")
	requireEqual(t, mock.destroyCalls, 3,
		"destroyTerrain calls destroy for each planet")
}

// TestDestroyTerrain_SkipsNeverProvisioned verifies that planets
// with no InstanceID and PlanetStatusLost are skipped.
func TestDestroyTerrain_SkipsNeverProvisioned(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	lost := testLostPlanet(1)
	lost.InstanceID = "" // never provisioned

	s.planets = []*Planet{
		lost,
		testProvisionedPlanet(2),
	}

	err := s.destroyTerrain()

	requireNoError(t, err, "destroyTerrain skips never provisioned")
	requireEqual(t, mock.destroyCalls, 1,
		"destroyTerrain only destroys provisioned planet")
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkProvisionPlanet measures the provisioning path with a
// mock provider -- excludes real terraform overhead.
func BenchmarkProvisionPlanet(b *testing.B) {
	mock := newMockProvider()
	s := benchmarkSovereign(b)
	s.provider = mock

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planet := &Planet{
			ID:           i + 1,
			Region:       testRegion,
			InstanceType: testInstanceType,
			Status:       PlanetStatusPending,
			RemoteUser:   "rocky",
		}
		s.provisionPlanet(planet)
	}
}

// BenchmarkDestroyPlanet measures the destroy path with a mock provider.
func BenchmarkDestroyPlanet(b *testing.B) {
	mock := newMockProvider()
	s := benchmarkSovereign(b)
	s.provider = mock

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planet := &Planet{
			ID:         i + 1,
			Region:     testRegion,
			InstanceID: fmt.Sprintf("i-%017d", i),
			Status:     PlanetStatusRunning,
			RemoteUser: "rocky",
		}
		s.destroyPlanet(planet)
	}
}

// -- helpers -----------------------------------------------------------------

// writeTerraformDir creates the terraform directory structure for a
// planet work dir, used in tests that need a pre-existing terraform dir.
func writeTerraformDir(t *testing.T, workDir string) {
	t.Helper()
	tfDir := filepath.Join(workDir, "terraform")
	if err := os.MkdirAll(tfDir, 0700); err != nil {
		t.Fatalf("writeTerraformDir: %v", err)
	}
	// write a minimal state file so Status() can find it
	stateFile := filepath.Join(tfDir, "terraform.tfstate")
	if err := os.WriteFile(stateFile, []byte(`{"version":4}`), 0600); err != nil {
		t.Fatalf("writeTerraformDir: write state: %v", err)
	}
}

// testProvisionedPlanetWithWorkDir returns a provisioned planet whose
// work directory has been created on disk.
func testProvisionedPlanetWithWorkDir(t *testing.T, s *Sovereign, id int) *Planet {
	t.Helper()
	planet := testProvisionedPlanet(id)
	writeTerraformDir(t, s.planetWorkDir(planet))
	return planet
}

// BenchmarkHCLSpec measures hclSpec construction throughput.
func BenchmarkHCLSpec(b *testing.B) {
	s := benchmarkSovereign(b)
	planet := &Planet{
		ID:           1,
		Region:       testRegion,
		InstanceType: testInstanceType,
		RemoteUser:   "rocky",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.hclSpec(planet)
	}
}

// BenchmarkPlanetSpec measures planetSpec construction throughput.
func BenchmarkPlanetSpec(b *testing.B) {
	s := benchmarkSovereign(b)
	planet := &Planet{
		ID:     1,
		Region: testRegion,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.planetSpec(planet)
	}
}

// timestampForTest returns a fixed timestamp for deterministic test output.
func timestampForTest() time.Time {
	return time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
}
