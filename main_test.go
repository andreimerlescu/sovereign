package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/commander"
	"github.com/andreimerlescu/sovereign/pkg/events"
	"github.com/andreimerlescu/sovereign/pkg/ledger"
)

// -- Constants ----------------------------------------------------------------

const (
	testVersion      = "test"
	testHit          = "http://localhost:8080/"
	testRegion       = "us-east-1"
	testInstanceType = "t3.medium"
	testZone         = "us-east-1a"
)

// -- Test config helpers ------------------------------------------------------

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

func testPlanet(id int) *Planet {
	return &Planet{
		ID:           id,
		Region:       testRegion,
		InstanceType: testInstanceType,
		RemoteUser:   defaultRemoteUser,
		Status:       PlanetStatusPending,
	}
}

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
		cmd:       commander.NewMockCommander(),
		out:       io.Discard,
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
// Variable names match the standard AWS SDK convention so operators
// with existing AWS tooling do not need additional configuration.
func setTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "test-aws-access-key-id")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-aws-secret-access-key")
	t.Setenv("SOVEREIGN_SUBNET_ID", "subnet-0test00000000000")
	t.Setenv("SOVEREIGN_SECURITY_GROUP", "sg-0test000000000000")
}

// -- Pricing mock server ------------------------------------------------------

func newMockPricingJSONServer(tb testing.TB, hourlyRateDollars float64) *httptest.Server {
	tb.Helper()
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
	tb.Cleanup(srv.Close)
	return srv
}

func newMockPricingJSONServerError(tb testing.TB) *httptest.Server {
	tb.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	tb.Cleanup(srv.Close)
	return srv
}

// -- Report file helpers ------------------------------------------------------

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
Duration: 30s
TotalVisits: %d
TotalBytes: 1048576
Total2xx: %d
Total3xx: 10
Total4xx: 5
Total5xx: 2
TotalWaitingRoom: 3
TotalErrors: 2
Fastest: 1ms
Slowest: 500ms
P50: 20ms
P90: 100ms
P95: 200ms
P99: 400ms
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
    P50: 18ms
    P99: 380ms
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

func testStateFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "state.yml")
}

// -- Assertion helpers --------------------------------------------------------

func requireNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", msg, err)
	}
}

func requireError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error but got nil", msg)
	}
}

func requireEqual[T comparable](t *testing.T, got, want T, msg string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %v, want %v", msg, got, want)
	}
}

func requireNotNil(t *testing.T, v any, msg string) {
	t.Helper()
	if v == nil {
		t.Fatalf("%s: expected non-nil value", msg)
	}
}

func requireTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Fatalf("%s: expected true", msg)
	}
}

func requireFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Fatalf("%s: expected false", msg)
	}
}

// -- Benchmark helpers --------------------------------------------------------

func benchmarkSovereign(b *testing.B) *Sovereign {
	b.Helper()
	cfg := testConfig()
	cfg.Planets = 1
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
		cmd:       commander.NewMockCommander(),
		out:       io.Discard,
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
