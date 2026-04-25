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

// -- resumeOrProvision -------------------------------------------------------

// TestResumeOrProvision_AlreadyCompleted verifies that a planet with a
// KindCollectComplete ledger entry is marked Done and skipped entirely.
func TestResumeOrProvision_AlreadyCompleted(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	// Simulate a completed planet by appending the ledger event directly.
	s.ledger.Append(events.Event{Kind: events.KindCollectComplete, PlanetID: 1})

	err := s.resumeOrProvision(planet)

	requireNoError(t, err, "resumeOrProvision completed planet is not an error")
	requireEqual(t, planet.Status, PlanetStatusDone,
		"resumeOrProvision sets PlanetStatusDone for completed planet")

	// The mock provider should not have been called at all.
	mock.mu.Lock()
	calls := mock.applyCalls + mock.initCalls + mock.outputCalls
	mock.mu.Unlock()
	requireEqual(t, calls, 0,
		"resumeOrProvision completed planet does not call provider")
}

// TestResumeOrProvision_AlreadyProvisioned verifies that a planet with a
// KindProvisionComplete ledger entry hydrates from provider state rather
// than re-provisioning.
func TestResumeOrProvision_AlreadyProvisioned(t *testing.T) {
	mock := newMockProvider()
	mock.stateResult = &tfprovider.ProviderState{
		Status:     tfprovider.LifecycleApplied,
		InstanceID: "i-resumed001",
		PublicIPv4: "54.9.9.1",
	}
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	s.ledger.Append(events.Event{Kind: events.KindProvisionComplete, PlanetID: 1})

	err := s.resumeOrProvision(planet)

	requireNoError(t, err, "resumeOrProvision provisioned planet resumes cleanly")
	requireEqual(t, planet.Status, PlanetStatusRunning,
		"resumeOrProvision provisioned planet is running")
	requireEqual(t, planet.InstanceID, "i-resumed001",
		"resumeOrProvision provisioned planet hydrates InstanceID")
	requireEqual(t, planet.PublicIPv4, "54.9.9.1",
		"resumeOrProvision provisioned planet hydrates PublicIPv4")

	// init and apply must not have been called.
	mock.mu.Lock()
	initCalls := mock.initCalls
	applyCalls := mock.applyCalls
	mock.mu.Unlock()
	requireEqual(t, initCalls, 0,
		"resumeOrProvision provisioned planet does not call Init")
	requireEqual(t, applyCalls, 0,
		"resumeOrProvision provisioned planet does not call Apply")
}

// TestResumeOrProvision_AlreadyProvisioned_MissingProviderState verifies
// that a planet marked provisioned in the ledger but with no provider
// state file on disk returns an error rather than silently proceeding.
func TestResumeOrProvision_AlreadyProvisioned_MissingProviderState(t *testing.T) {
	mock := newMockProvider()
	mock.stateResult = nil
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	s.ledger.Append(events.Event{Kind: events.KindProvisionComplete, PlanetID: 1})

	err := s.resumeOrProvision(planet)

	requireError(t, err, "resumeOrProvision provisioned but missing state returns error")
}

// TestResumeOrProvision_InterruptedApply verifies that a planet in
// LifecycleApplying state re-runs apply rather than starting fresh.
func TestResumeOrProvision_InterruptedApply(t *testing.T) {
	mock := newMockProvider()
	mock.stateResult = &tfprovider.ProviderState{
		Status: tfprovider.LifecycleApplying,
	}
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	err := s.resumeOrProvision(planet)

	requireNoError(t, err, "resumeOrProvision interrupted apply succeeds")
	requireEqual(t, planet.Status, PlanetStatusRunning,
		"resumeOrProvision interrupted apply reaches running")

	// apply must have been called; init must not have been.
	mock.mu.Lock()
	applyCalls := mock.applyCalls
	initCalls := mock.initCalls
	mock.mu.Unlock()
	requireEqual(t, applyCalls, 1,
		"resumeOrProvision interrupted apply calls Apply once")
	requireEqual(t, initCalls, 0,
		"resumeOrProvision interrupted apply does not call Init")
}

// TestResumeOrProvision_InterruptedState verifies that LifecycleInterrupted
// also re-runs apply.
func TestResumeOrProvision_InterruptedState(t *testing.T) {
	mock := newMockProvider()
	mock.stateResult = &tfprovider.ProviderState{
		Status: tfprovider.LifecycleInterrupted,
	}
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	err := s.resumeOrProvision(planet)

	requireNoError(t, err, "resumeOrProvision interrupted state succeeds")

	mock.mu.Lock()
	applyCalls := mock.applyCalls
	mock.mu.Unlock()
	requireEqual(t, applyCalls, 1,
		"resumeOrProvision LifecycleInterrupted calls Apply")
}

// TestResumeOrProvision_Initialized verifies that a planet in
// LifecycleInitialized state skips init and runs apply.
func TestResumeOrProvision_Initialized(t *testing.T) {
	mock := newMockProvider()
	mock.stateResult = &tfprovider.ProviderState{
		Status: tfprovider.LifecycleInitialized,
	}
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	err := s.resumeOrProvision(planet)

	requireNoError(t, err, "resumeOrProvision initialized state succeeds")

	mock.mu.Lock()
	initCalls := mock.initCalls
	applyCalls := mock.applyCalls
	mock.mu.Unlock()
	requireEqual(t, initCalls, 0,
		"resumeOrProvision initialized skips Init")
	requireEqual(t, applyCalls, 1,
		"resumeOrProvision initialized calls Apply")
}

// TestResumeOrProvision_AlreadyApplied verifies that a planet in
// LifecycleApplied state hydrates from the provider state without
// calling init or apply.
func TestResumeOrProvision_AlreadyApplied(t *testing.T) {
	mock := newMockProvider()
	mock.stateResult = &tfprovider.ProviderState{
		Status:     tfprovider.LifecycleApplied,
		InstanceID: "i-applied001",
		PublicIPv4: "54.8.8.1",
	}
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	err := s.resumeOrProvision(planet)

	requireNoError(t, err, "resumeOrProvision applied state succeeds")
	requireEqual(t, planet.InstanceID, "i-applied001",
		"resumeOrProvision applied hydrates InstanceID")
	requireEqual(t, planet.PublicIPv4, "54.8.8.1",
		"resumeOrProvision applied hydrates PublicIPv4")

	mock.mu.Lock()
	initCalls := mock.initCalls
	applyCalls := mock.applyCalls
	mock.mu.Unlock()
	requireEqual(t, initCalls, 0, "resumeOrProvision applied does not call Init")
	requireEqual(t, applyCalls, 0, "resumeOrProvision applied does not call Apply")
}

// TestResumeOrProvision_FreshPlanet verifies that a planet with no
// ledger history and no provider state provisions from scratch.
func TestResumeOrProvision_FreshPlanet(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	err := s.resumeOrProvision(planet)

	requireNoError(t, err, "resumeOrProvision fresh planet succeeds")
	requireEqual(t, planet.Status, PlanetStatusRunning,
		"resumeOrProvision fresh planet reaches running")

	mock.mu.Lock()
	initCalls := mock.initCalls
	applyCalls := mock.applyCalls
	mock.mu.Unlock()
	requireTrue(t, initCalls > 0, "resumeOrProvision fresh planet calls Init")
	requireTrue(t, applyCalls > 0, "resumeOrProvision fresh planet calls Apply")
}

// TestResumeOrProvision_InterruptedApply_EmitsRunInterrupted verifies
// that the KindRunInterrupted event is emitted when resuming an
// interrupted apply.
func TestResumeOrProvision_InterruptedApply_EmitsRunInterrupted(t *testing.T) {
	mock := newMockProvider()
	mock.stateResult = &tfprovider.ProviderState{
		Status: tfprovider.LifecycleApplying,
	}
	s := testSovereignWithMockProvider(t, mock)
	planet := testPlanet(1)

	var received []events.EventKind
	var mu sync.Mutex
	s.bus.Subscribe(func(e events.Event) {
		if e.PlanetID == planet.ID {
			mu.Lock()
			received = append(received, e.Kind)
			mu.Unlock()
		}
	})

	s.resumeOrProvision(planet)

	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, k := range received {
		if k == events.KindRunInterrupted {
			found = true
			break
		}
	}
	requireTrue(t, found, "resumeOrProvision interrupted emits KindRunInterrupted")
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

// TestProvisionTerrain_ContextCancellation verifies that cancelling the
// context while the semaphore is full does not leave goroutines stranded
// and that provisionTerrain returns promptly.
func TestProvisionTerrain_ContextCancellation(t *testing.T) {
	mock := newMockProvider()
	// Block apply indefinitely so goroutines hold the semaphore slots.
	blocked := make(chan struct{})
	mock.applyErr = nil

	ctx, cancel := context.WithCancel(context.Background())

	s := testSovereign(t)
	s.ctx = ctx
	s.provider = mock
	s.cfg.Planets = 5
	s.cfg.Regions = []string{testRegion}
	s.planets = s.initPlanets()

	// Apply blocks until we cancel.
	origApply := mock.applyErr
	_ = origApply
	applyStarted := make(chan struct{}, 10)

	// Replace the mock's Apply with one that signals and then blocks.
	blockingMock := &blockingMockProvider{
		MockTerraformProvider: mock,
		applyStarted:          applyStarted,
		blocked:               blocked,
	}
	s.provider = blockingMock

	done := make(chan error, 1)
	go func() {
		done <- s.provisionTerrain()
	}()

	// Wait for at least one apply to start, then cancel.
	select {
	case <-applyStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for apply to start")
	}
	cancel()
	close(blocked)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("provisionTerrain did not return after context cancellation")
	}
}

// blockingMockProvider wraps MockTerraformProvider with a blocking Apply.
type blockingMockProvider struct {
	*MockTerraformProvider
	applyStarted chan struct{}
	blocked      chan struct{}
}

func (b *blockingMockProvider) Apply(ctx context.Context, spec tfprovider.PlanetSpec) error {
	select {
	case b.applyStarted <- struct{}{}:
	default:
	}
	select {
	case <-b.blocked:
	case <-ctx.Done():
	}
	return ctx.Err()
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

// TestDestroyTerrain_SkipsNeverProvisioned verifies that a planet with
// no InstanceID is skipped regardless of its Status. Previously a lost
// planet with Status==PlanetStatusLost&&InstanceID=="" was the check;
// the correct check is InstanceID=="" alone.
func TestDestroyTerrain_SkipsNeverProvisioned(t *testing.T) {
	mock := newMockProvider()
	s := testSovereignWithMockProvider(t, mock)

	// Lost planet with no InstanceID -- never provisioned.
	neverProvisioned := testLostPlanet(1)
	neverProvisioned.InstanceID = ""

	// Pending planet with no InstanceID -- also skipped.
	pending := testPlanet(2)

	// Lost planet WITH an InstanceID -- was provisioned, must be destroyed.
	lostButProvisioned := testLostPlanet(3)
	lostButProvisioned.InstanceID = "i-was-running-but-lost"

	s.planets = []*Planet{
		neverProvisioned,
		pending,
		lostButProvisioned,
		testProvisionedPlanet(4),
	}

	err := s.destroyTerrain()

	requireNoError(t, err, "destroyTerrain skips never provisioned")
	mock.mu.Lock()
	calls := mock.destroyCalls
	mock.mu.Unlock()
	requireEqual(t, calls, 2,
		"destroyTerrain destroys lostButProvisioned and running planet, skips the rest")
}

// -- Benchmarks --------------------------------------------------------------

func BenchmarkProvisionPlanet(b *testing.B) {
	mock := newMockProvider()
	s := benchmarkSovereign(b)
	s.provider = mock
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planet := &Planet{
			ID:           1,
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
			ID:         1,
			Region:     testRegion,
			InstanceID: "i-00000000000000001",
			Status:     PlanetStatusRunning,
			RemoteUser: "rocky",
		}
		s.destroyPlanet(planet)
	}
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

func timestampForTest() time.Time {
	return time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
}
