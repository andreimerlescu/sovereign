package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/events"
	"github.com/andreimerlescu/sovereign/pkg/ledger"
)

// -- Constants ----------------------------------------------------------------

const (
	// testVersion is the sovereign version used in tests.
	testVersion = "test"

	// testHit is the default target URL used in tests.
	testHit = "http://localhost:8080/"

	// testRegion is the default AWS region used in tests.
	testRegion = "us-east-1"

	// testInstanceType is the default AWS instance type used in tests.
	testInstanceType = "t3.medium"

	// testZone is the default AWS availability zone used in tests.
	testZone = "us-east-1a"
)

// -- Commander interface ------------------------------------------------------

// Commander wraps exec.CommandContext so tests can inject a fake
// implementation that returns controlled output without spawning
// real processes. Production code uses RealCommander. Tests use
// MockCommander.
type Commander interface {
	Run(ctx context.Context, name string, args ...string) (stdout, stderr string, err error)
}

// RealCommander executes real OS commands via os/exec.
type RealCommander struct{}

// Run executes the named command and returns its combined output.
func (r *RealCommander) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	return "", "", fmt.Errorf("RealCommander.Run: not implemented in test context")
}

// MockCommander is a test double for Commander that returns
// pre-configured responses without executing real processes.
type MockCommander struct {
	mu       sync.Mutex
	calls    []MockCall
	handlers map[string]MockHandler
}

// MockCall records a single command invocation for later assertion.
type MockCall struct {
	Name string
	Args []string
}

// MockHandler defines the response for a registered command.
type MockHandler struct {
	Stdout string
	Stderr string
	Err    error
}

// NewMockCommander constructs a MockCommander with no registered handlers.
func NewMockCommander() *MockCommander {
	return &MockCommander{
		handlers: make(map[string]MockHandler),
	}
}

// Register sets the response for a command identified by its name.
func (m *MockCommander) Register(name string, handler MockHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[name] = handler
}

// Run records the call and returns the registered handler response.
func (m *MockCommander) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	m.mu.Lock()
	m.calls = append(m.calls, MockCall{Name: name, Args: args})
	handler, ok := m.handlers[name]
	m.mu.Unlock()
	if !ok {
		return "ok", "", nil
	}
	return handler.Stdout, handler.Stderr, handler.Err
}

// Called returns true if the named command was invoked at least once.
func (m *MockCommander) Called(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.calls {
		if c.Name == name {
			return true
		}
	}
	return false
}

// CallCount returns the number of times the named command was invoked.
func (m *MockCommander) CallCount(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, c := range m.calls {
		if c.Name == name {
			count++
		}
	}
	return count
}

// Calls returns all recorded invocations of the named command.
func (m *MockCommander) Calls(name string) []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []MockCall
	for _, c := range m.calls {
		if c.Name == name {
			result = append(result, c)
		}
	}
	return result
}

// Reset clears all recorded calls and registered handlers.
func (m *MockCommander) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
	m.handlers = make(map[string]MockHandler)
}

// -- Test config helpers ------------------------------------------------------

// testConfig returns a minimal valid SovereignConfig for use in tests.
func testConfig() SovereignConfig {
	return SovereignConfig{
		Hit:              testHit,
		Planets:          2,
		Terrain:          2,
		Pack:             2,
		Until:            30 * time.Second,
		Ramp:             5 * time.Second,
		Plan:             "",
		Regions:          []string{testRegion},
		InstanceType:     testInstanceType,
		SaveTo:           []string{"."},
		ProvisionTimeout: 5 * time.Minute,
		TerminateTimeout: 5 * time.Minute,
		Yes:              true,
		DryRun:           false,
		Version:          testVersion,
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
}

// -- Planet helpers -----------------------------------------------------------

// testPlanet returns a Planet in PlanetStatusPending for use in tests.
func testPlanet(id int) *Planet {
	return &Planet{
		ID:           id,
		Region:       testRegion,
		InstanceType: testInstanceType,
		RemoteUser:   defaultRemoteUser,
		Status:       PlanetStatusPending,
	}
}

// testProvisionedPlanet returns a Planet in PlanetStatusRunning
// with PublicIPv4 and InstanceID populated.
func testProvisionedPlanet(id int) *Planet {
	return &Planet{
		ID:            id,
		Region:        testRegion,
		InstanceType:  testInstanceType,
		RemoteUser:    defaultRemoteUser,
		PublicIPv4:    fmt.Sprintf("54.%d.%d.1", id, id*10),
		InstanceID:    fmt.Sprintf("i-0000000000000000%d", id),
		Status:        PlanetStatusRunning,
		ProvisionedAt: time.Now(),
	}
}

// testDonePlanet returns a Planet in PlanetStatusDone with a
// ReportPath pointing to a temp directory.
func testDonePlanet(t *testing.T, id int) *Planet {
	t.Helper()
	dir := t.TempDir()
	return &Planet{
		ID:            id,
		Region:        testRegion,
		InstanceType:  testInstanceType,
		RemoteUser:    defaultRemoteUser,
		PublicIPv4:    fmt.Sprintf("54.%d.%d.1", id, id*10),
		InstanceID:    fmt.Sprintf("i-0000000000000000%d", id),
		Status:        PlanetStatusDone,
		ProvisionedAt: time.Now().Add(-10 * time.Minute),
		CompletedAt:   time.Now(),
		ReportPath:    dir,
	}
}

// testLostPlanet returns a Planet in PlanetStatusLost.
func testLostPlanet(id int) *Planet {
	return &Planet{
		ID:           id,
		Region:       testRegion,
		InstanceType: testInstanceType,
		RemoteUser:   defaultRemoteUser,
		Status:       PlanetStatusLost,
		Error:        fmt.Errorf("simulated provisioning failure"),
	}
}

// -- Sovereign helpers --------------------------------------------------------

// testSovereign constructs a Sovereign with a test config, a temp
// directory for state, an initialized event bus, and a ledger wired
// as a bus subscriber.
func testSovereign(t *testing.T) *Sovereign {
	t.Helper()
	cfg := testConfig()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	stateDir := t.TempDir()
	stateFile := filepath.Join(stateDir, "state.yml")

	bus := events.NewEventBus()
	t.Cleanup(bus.Close)

	l, err := ledger.NewLedger(stateDir, time.Now())
	if err != nil {
		t.Fatalf("testSovereign: ledger.NewLedger: %v", err)
	}
	bus.Subscribe(l.AsSubscriber())

	s := &Sovereign{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		startedAt: time.Now(),
		stateFile: stateFile,
		bus:       bus,
		ledger:    l,
	}
	s.planets = s.initPlanets()
	s.state = &SovereignState{
		Version:   cfg.Version,
		StartedAt: s.startedAt,
		Config:    cfg,
		Planets:   s.planets,
	}
	return s
}

// testSovereignWithPlanets constructs a Sovereign with the provided
// planets already populated.
func testSovereignWithPlanets(t *testing.T, planets []*Planet) *Sovereign {
	t.Helper()
	s := testSovereign(t)
	s.planets = planets
	s.state.Planets = planets
	return s
}

// -- Environment helpers ------------------------------------------------------

// setTestEnv sets required sovereign environment variables using
// t.Setenv so they are automatically restored after each test.
func setTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY", "test-aws-access-key")
	t.Setenv("AWS_ACCESS_SECRET_KEY", "test-aws-secret-key")
	t.Setenv("SOVEREIGN_SUBNET_ID", "subnet-0test00000000000")
	t.Setenv("SOVEREIGN_SECURITY_GROUP", "sg-0test000000000000")
}

// -- Pricing mock server ------------------------------------------------------

// newMockPricingJSONServer starts an httptest.Server that returns a
// fixed pricing JSON response for any instance type and region.
// Used by cost_test.go to test estimateCost against a real HTTP server
// without hitting instances.vantage.sh.
//
// The response format matches what VantagePricer expects from the API.
// instance_type and region are reflected from the query parameters.
func newMockPricingJSONServer(t *testing.T, hourlyRateDollars float64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		instanceType := r.URL.Query().Get("instance_type")
		if instanceType == "" {
			instanceType = testInstanceType
		}
		region := r.URL.Query().Get("region")
		if region == "" {
			region = testRegion
		}
		body := fmt.Sprintf(
			`[{"instance_type":%q,"region":%q,"on_demand":%f}]`,
			instanceType, region, hourlyRateDollars,
		)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newMockPricingJSONServerError starts an httptest.Server that returns
// HTTP 500 for all requests.
func newMockPricingJSONServerError(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// -- Report file helpers ------------------------------------------------------

// writeMockLemmingsReport writes a minimal lemmings YAML report file
// into dir so that readPlanetReport has something to parse.
func writeMockLemmingsReport(t *testing.T, dir string, planetID int) string {
	t.Helper()
	domain := "localhost-8080"
	date := time.Now().Format("2006.01.02")
	filename := fmt.Sprintf("lemmings.%s.%s.yml", date, domain)

	subDir := filepath.Join(dir, "lemmings", domain)
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("writeMockLemmingsReport: mkdir %s: %v", subDir, err)
	}

	contents := fmt.Sprintf(`Version: test
GeneratedAt: %s
Duration: 30000000000
TotalVisits: %d
TotalBytes: 1048576
Total2xx: %d
Total3xx: 10
Total4xx: 5
Total5xx: 2
TotalWaitingRoom: 3
TotalErrors: 2
Fastest: 1000000
Slowest: 500000000
P50: 20000000
P90: 100000000
P95: 200000000
P99: 400000000
Paths:
  - URL: http://localhost:8080/
    Hits: %d
    Bytes: 524288
    XX2: %d
    XX3: 5
    XX4: 3
    XX5: 1
    WaitingRoom: 1
    Errors: 1
    P50: 18000000
    P99: 380000000
`,
		time.Now().Format(time.RFC3339),
		100*planetID,
		85*planetID,
		60*planetID,
		55*planetID,
	)

	path := filepath.Join(subDir, filename)
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("writeMockLemmingsReport: write %s: %v", path, err)
	}
	return path
}

// -- State file helpers -------------------------------------------------------

// testStateFile returns a path to a temp state file.
func testStateFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "state.yml")
}

// -- Assertion helpers --------------------------------------------------------

// requireNoError fails the test immediately if err is non-nil.
func requireNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", msg, err)
	}
}

// requireError fails the test immediately if err is nil.
func requireError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", msg)
	}
}

// requireEqual fails the test if got != want.
func requireEqual[T comparable](t *testing.T, got, want T, msg string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %v, want %v", msg, got, want)
	}
}

// requireNotNil fails the test if v is nil.
func requireNotNil(t *testing.T, v any, msg string) {
	t.Helper()
	if v == nil {
		t.Fatalf("%s: expected non-nil value", msg)
	}
}

// requireTrue fails the test if condition is false.
func requireTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Fatalf("%s: expected true", msg)
	}
}

// requireFalse fails the test if condition is true.
func requireFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Fatalf("%s: expected false", msg)
	}
}

// -- Benchmark helpers --------------------------------------------------------

// benchmarkSovereign constructs a Sovereign suitable for benchmark use.
func benchmarkSovereign(b *testing.B) *Sovereign {
	b.Helper()
	cfg := testConfig()
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(cancel)

	stateDir := b.TempDir()
	stateFile := filepath.Join(stateDir, "state.yml")

	bus := events.NewEventBus()
	b.Cleanup(bus.Close)

	l, err := ledger.NewLedger(stateDir, time.Now())
	if err != nil {
		b.Fatalf("benchmarkSovereign: ledger.NewLedger: %v", err)
	}
	bus.Subscribe(l.AsSubscriber())

	s := &Sovereign{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		startedAt: time.Now(),
		stateFile: stateFile,
		bus:       bus,
		ledger:    l,
	}
	s.planets = s.initPlanets()
	s.state = &SovereignState{
		Version:   cfg.Version,
		StartedAt: s.startedAt,
		Config:    cfg,
		Planets:   s.planets,
	}
	return s
}
