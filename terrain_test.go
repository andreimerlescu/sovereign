package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/events"
	tfprovider "github.com/andreimerlescu/sovereign/pkg/terraform/provider"
)

// -- MockTerraformProvider ---------------------------------------------------

type MockTerraformProvider struct {
	mu            sync.Mutex
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

func (m *MockTerraformProvider) Name() string { return m.name }
func (m *MockTerraformProvider) Validate() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.validateErr
}

func (m *MockTerraformProvider) RenderHCL(spec tfprovider.HCLSpec) error {
	m.mu.Lock()
	m.renderedSpecs = append(m.renderedSpecs, spec)
	err := m.renderErr
	m.mu.Unlock()
	return err
}

func (m *MockTerraformProvider) Init(_ context.Context, _ tfprovider.PlanetSpec) error {
	m.mu.Lock()
	m.initCalls++
	err := m.initErr
	m.mu.Unlock()
	return err
}

func (m *MockTerraformProvider) InitOrApply(_ context.Context, _ tfprovider.PlanetSpec) error {
	m.mu.Lock()
	m.initCalls++
	m.applyCalls++
	initErr := m.initErr
	applyErr := m.applyErr
	m.mu.Unlock()
	if initErr != nil {
		return initErr
	}
	return applyErr
}

func (m *MockTerraformProvider) Apply(_ context.Context, _ tfprovider.PlanetSpec) error {
	m.mu.Lock()
	m.applyCalls++
	err := m.applyErr
	m.mu.Unlock()
	return err
}

func (m *MockTerraformProvider) Destroy(_ context.Context, _ tfprovider.PlanetSpec) error {
	m.mu.Lock()
	m.destroyCalls++
	err := m.destroyErr
	m.mu.Unlock()
	return err
}

func (m *MockTerraformProvider) Output(_ context.Context, _ tfprovider.PlanetSpec) (*tfprovider.ProvisionOutput, error) {
	m.mu.Lock()
	m.outputCalls++
	result := m.outputResult
	err := m.outputErr
	m.mu.Unlock()
	return result, err
}

func (m *MockTerraformProvider) Status(_ context.Context, _ tfprovider.PlanetSpec) (tfprovider.LifecycleStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusResult, m.statusErr
}

func (m *MockTerraformProvider) StateBackend(spec tfprovider.PlanetSpec) (string, error) {
	return filepath.Join(spec.WorkDir, "terraform", "terraform.tfstate"), nil
}

func (m *MockTerraformProvider) LoadState(_ tfprovider.PlanetSpec) (*tfprovider.ProviderState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stateResult, m.stateErr
}

func (m *MockTerraformProvider) SaveState(_ tfprovider.PlanetSpec, state *tfprovider.ProviderState) error {
	m.mu.Lock()
	m.savedStates = append(m.savedStates, state)
	m.mu.Unlock()
	return nil
}

func (m *MockTerraformProvider) UnloadState(_ tfprovider.PlanetSpec) error {
	return nil
}

// -- test helper -------------------------------------------------------------

func testSovereignWithMockProvider(t *testing.T, mock *MockTerraformProvider) *Sovereign {
	t.Helper()
	s := testSovereign(t)
	s.provider = mock
	return s
}

// -- planetSpec --------------------------------------------------------------

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

func TestPlanetSpec_ZoneDerivesFromRegion(t *testing.T) {
	s := testSovereign(t)
	planet := testPlanet(1)
	planet.Region = "us-west-2"

	spec := s.planetSpec(planet)
	requireEqual(t, spec.Zone, "us-west-2a", "planetSpec Zone derived from region")
}

// -- hclSpec -----------------------------------------------------------------

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

func TestHCLSpec_SlugContainsPlanetID(t *testing.T) {
	s := testSovereign(t)
	planet := testPlanet(7)
	spec := s.hclSpec(planet)
	requireTrue(t, containsStr(spec.Slug, "7"),
		"hclSpec Slug contains planet ID")
}

// -- markLost ----------------------------------------------------------------

func TestMarkLost_SetsPlanetStatusLost(t *testing.T) {
	s := testSovereignWithMockProvider(t, newMockProvider())
	planet := testPlanet(1)

	err := s.markLost(planet, events.KindProvisionFailed,
		fmt.Errorf("simulated failure"))

	requireError(t, err, "markLost returns error")
	requireEqual(t, planet.Status, PlanetStatusLost,
		"markLost sets PlanetStatusLost")
}

func TestMarkLost_AppendsLedgerEvent(t *testing.T) {
	s := testSovereignWithMockProvider(t, newMockProvider())
	planet := testPlanet(1)

	s.markLost(planet, events.KindProvisionFailed,
		fmt.Errorf("simulated failure"))

	last := s.ledger.LastEntryForPlanet(1)
	requireTrue(t, last != nil, "markLost appends ledger event")
	requireEqual(t, last.Kind, events.KindProvisionFailed,
		"markLost appends correct event type")
}

func TestMarkLost_SetsErrorOnPlanet(t *testing.T) {
	s := testSovereignWithMockProvider(t, newMockProvider())
	planet := testPlanet(1)
	simErr := fmt.Errorf("simulated failure")

	s.markLost(planet, events.KindProvisionFailed, simErr)

	requireTrue(t, planet.Error != nil, "markLost sets planet.Error")
}

// -- saveProviderState -------------------------------------------------------

func TestSaveProviderState_CallsProviderSaveState(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	s.saveProviderState(planet, tfprovider.LifecycleApplied, nil)

	mock.mu.Lock()
	count := len(mock.savedStates)
	mock.mu.Unlock()
	requireEqual(t, count, 1,
		"saveProviderState calls provider SaveState")
}

func TestSaveProviderState_StatusPreserved(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	s.saveProviderState(planet, tfprovider.LifecycleInitialized, nil)

	mock.mu.Lock()
	status := mock.savedStates[0].Status
	mock.mu.Unlock()
	requireEqual(t, status, tfprovider.LifecycleInitialized,
		"saveProviderState status preserved")
}

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

	mock.mu.Lock()
	saved := mock.savedStates[0]
	mock.mu.Unlock()
	requireEqual(t, saved.InstanceID, "i-abc123",
		"saveProviderState InstanceID from output")
	requireEqual(t, saved.PublicIPv4, "1.2.3.4",
		"saveProviderState PublicIPv4 from output")
}

// -- provisionPlanet ---------------------------------------------------------

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

func TestProvisionPlanet_LedgerRecordsProvisionComplete(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	s.provisionPlanet(planet)

	requireTrue(t, s.ledger.PlanetProvisioned(1),
		"provisionPlanet ledger records provision complete")
}

// -- destroyPlanet -----------------------------------------------------------

func TestDestroyPlanet_Success(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testProvisionedPlanet(1)

	err := s.destroyPlanet(planet)

	requireNoError(t, err, "destroyPlanet success")
	mock.mu.Lock()
	calls := mock.destroyCalls
	mock.mu.Unlock()
	requireEqual(t, calls, 1,
		"destroyPlanet calls provider Destroy once")
}

func TestDestroyPlanet_FailureReturnsError(t *testing.T) {
	mock := newMockProvider()
	mock.destroyErr = fmt.Errorf("destroy failed")
	s := testSovereignWithMockProvider(t, mock)
	planet := testProvisionedPlanet(1)

	err := s.destroyPlanet(planet)

	requireError(t, err, "destroyPlanet failure returns error")
}

func TestDestroyPlanet_LedgerRecordsDestroyComplete(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testProvisionedPlanet(1)

	s.destroyPlanet(planet)

	last := s.ledger.LastEntryForPlanet(1)
	requireTrue(t, last != nil, "destroyPlanet ledger event exists")
	requireEqual(t, last.Kind, events.KindDestroyComplete,
		"destroyPlanet ledger records destroy complete")
}

// -- planetWorkDir -----------------------------------------------------------

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

func TestProvisionTerrain_EmptyPlanetsReturnsError(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	s.planets = []*Planet{}

	err := s.provisionTerrain()

	requireError(t, err, "provisionTerrain empty planets")
}

func TestProvisionTerrain_FirstPlanetFailureReturnsError(t *testing.T) {
	mock := newMockProvider()
	mock.initErr = fmt.Errorf("init failed")
	s := testSovereignWithMockProvider(t, mock)
	s.cfg.Planets = 3
	s.planets = s.initPlanets()

	err := s.provisionTerrain()

	requireError(t, err, "provisionTerrain first planet failure")
}

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
	mock.mu.Lock()
	calls := mock.destroyCalls
	mock.mu.Unlock()
	requireEqual(t, calls, 3,
		"destroyTerrain calls destroy for each planet")
}

func TestDestroyTerrain_SkipsNeverProvisioned(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	lost := testLostPlanet(1)
	lost.InstanceID = ""

	s.planets = []*Planet{
		lost,
		testProvisionedPlanet(2),
	}

	err := s.destroyTerrain()

	requireNoError(t, err, "destroyTerrain skips never provisioned")
	mock.mu.Lock()
	calls := mock.destroyCalls
	mock.mu.Unlock()
	requireEqual(t, calls, 1,
		"destroyTerrain only destroys provisioned planet")
}

// -- Benchmarks --------------------------------------------------------------

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

func writeTerraformDir(t *testing.T, workDir string) {
	t.Helper()
	tfDir := filepath.Join(workDir, "terraform")
	if err := os.MkdirAll(tfDir, 0700); err != nil {
		t.Fatalf("writeTerraformDir: %v", err)
	}
	stateFile := filepath.Join(tfDir, "terraform.tfstate")
	if err := os.WriteFile(stateFile, []byte(`{"version":4}`), 0600); err != nil {
		t.Fatalf("writeTerraformDir: write state: %v", err)
	}
}

func testProvisionedPlanetWithWorkDir(t *testing.T, s *Sovereign, id int) *Planet {
	t.Helper()
	planet := testProvisionedPlanet(id)
	writeTerraformDir(t, s.planetWorkDir(planet))
	return planet
}

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

func timestampForTest() time.Time {
	return time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
}
