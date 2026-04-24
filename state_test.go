package main

import (
	"os"
	"path/filepath"
	"testing"
)

// -- saveState / loadState ---------------------------------------------------

// TestSaveState_WritesFile verifies that saveState creates the state
// file on disk.
func TestSaveState_WritesFile(t *testing.T) {
	s := testSovereign(t)
	err := s.saveState()
	requireNoError(t, err, "saveState writes file")

	if _, statErr := os.Stat(s.stateFile); statErr != nil {
		t.Fatalf("saveState: state file not found at %s", s.stateFile)
	}
}

// TestSaveState_LoadState_RoundTrip verifies that state written by
// saveState can be read back by loadState with the planet slice intact.
func TestSaveState_LoadState_RoundTrip(t *testing.T) {
	s := testSovereign(t)
	s.planets = []*Planet{
		testProvisionedPlanet(1),
		testProvisionedPlanet(2),
	}
	s.state.Planets = s.planets

	requireNoError(t, s.saveState(), "saveState round trip save")

	loaded, err := s.loadState()
	requireNoError(t, err, "loadState round trip load")
	requireTrue(t, loaded != nil, "loadState round trip non-nil")
	requireEqual(t, len(loaded.Planets), 2,
		"loadState round trip planet count")
	requireEqual(t, loaded.Planets[0].ID, 1,
		"loadState round trip planet 1 ID")
	requireEqual(t, loaded.Planets[1].ID, 2,
		"loadState round trip planet 2 ID")
}

// TestLoadState_NilWhenNoFile verifies that loadState returns nil, nil
// when no state file exists.
func TestLoadState_NilWhenNoFile(t *testing.T) {
	s := testSovereign(t)
	// stateFile set to a path that does not exist
	s.stateFile = filepath.Join(t.TempDir(), "nonexistent", "state.yml")

	loaded, err := s.loadState()
	requireNoError(t, err, "loadState nil when no file")
	requireTrue(t, loaded == nil, "loadState returns nil for missing file")
}

// TestLoadState_ErrorOnMalformedFile verifies that a malformed YAML
// state file returns a non-nil error.
func TestLoadState_ErrorOnMalformedFile(t *testing.T) {
	s := testSovereign(t)
	err := os.WriteFile(s.stateFile, []byte("{ not valid yaml :::"), 0600)
	requireNoError(t, err, "loadState write malformed file")

	_, loadErr := s.loadState()
	requireError(t, loadErr, "loadState error on malformed file")
}

// TestSaveState_PlanetsPreserved verifies that planet fields are
// preserved through a save/load cycle.
func TestSaveState_PlanetsPreserved(t *testing.T) {
	s := testSovereign(t)
	planet := testProvisionedPlanet(1)
	planet.PublicIPv4 = "54.1.2.3"
	planet.InstanceID = "i-abc123"
	s.planets = []*Planet{planet}
	s.state.Planets = s.planets

	requireNoError(t, s.saveState(), "saveState planets preserved save")

	loaded, err := s.loadState()
	requireNoError(t, err, "saveState planets preserved load")
	requireEqual(t, loaded.Planets[0].PublicIPv4, "54.1.2.3",
		"saveState PublicIPv4 preserved")
	requireEqual(t, loaded.Planets[0].InstanceID, "i-abc123",
		"saveState InstanceID preserved")
}

// -- cleanState --------------------------------------------------------------

// TestCleanState_RemovesFile verifies that cleanState removes the
// state file from disk.
func TestCleanState_RemovesFile(t *testing.T) {
	s := testSovereign(t)
	requireNoError(t, s.saveState(), "cleanState save first")

	s.cleanState()

	if _, err := os.Stat(s.stateFile); !os.IsNotExist(err) {
		t.Fatalf("cleanState: state file still exists at %s", s.stateFile)
	}
}

// TestCleanState_NoErrorWhenFileAbsent verifies that cleanState does
// not panic when the state file does not exist.
func TestCleanState_NoErrorWhenFileAbsent(t *testing.T) {
	s := testSovereign(t)
	s.stateFile = filepath.Join(t.TempDir(), "nonexistent.yml")
	// must not panic
	s.cleanState()
}

// -- resolveStateFile --------------------------------------------------------

// TestResolveStateFile_ReturnsPath verifies that resolveStateFile
// returns a non-empty path.
func TestResolveStateFile_ReturnsPath(t *testing.T) {
	s := testSovereign(t)
	path, err := s.resolveStateFile()
	requireNoError(t, err, "resolveStateFile returns path")
	requireTrue(t, len(path) > 0, "resolveStateFile non-empty path")
}

// TestResolveStateFile_ContainsStateYML verifies that the returned
// path ends with state.yml.
func TestResolveStateFile_ContainsStateYML(t *testing.T) {
	s := testSovereign(t)
	path, err := s.resolveStateFile()
	requireNoError(t, err, "resolveStateFile contains state.yml")
	requireEqual(t, filepath.Base(path), "state.yml",
		"resolveStateFile ends with state.yml")
}

// -- planetState -------------------------------------------------------------

// TestPlanetState_SnapshotIsIndependent verifies that the snapshot
// returned by planetState is independent of the original planet --
// modifying the original does not affect the snapshot.
func TestPlanetState_SnapshotIsIndependent(t *testing.T) {
	planet := testProvisionedPlanet(1)
	snapshot := planetState(planet)

	// modify original
	planet.PublicIPv4 = "99.99.99.99"

	requireEqual(t, snapshot.PublicIPv4, "54.1.10.1",
		"planetState snapshot independent of original")
}

// TestPlanetState_CopiesAllFields verifies that all exported fields
// are copied into the snapshot.
func TestPlanetState_CopiesAllFields(t *testing.T) {
	planet := testProvisionedPlanet(1)
	planet.ReportPath = "/tmp/reports"
	snapshot := planetState(planet)

	requireEqual(t, snapshot.ID, planet.ID, "planetState ID copied")
	requireEqual(t, snapshot.Region, planet.Region, "planetState Region copied")
	requireEqual(t, snapshot.InstanceType, planet.InstanceType,
		"planetState InstanceType copied")
	requireEqual(t, snapshot.PublicIPv4, planet.PublicIPv4,
		"planetState PublicIPv4 copied")
	requireEqual(t, snapshot.InstanceID, planet.InstanceID,
		"planetState InstanceID copied")
	requireEqual(t, snapshot.Status, planet.Status,
		"planetState Status copied")
	requireEqual(t, snapshot.ReportPath, planet.ReportPath,
		"planetState ReportPath copied")
}

// -- lostPlanets / pendingPlanets / allPlanetsTerminal -----------------------

// TestStateLostPlanets_Empty verifies an empty slice when no planets
// are lost.
func TestStateLostPlanets_Empty(t *testing.T) {
	s := testSovereignWithPlanets(t, []*Planet{
		testPlanet(1),
		testDonePlanet(t, 2),
	})
	requireEqual(t, len(s.lostPlanets()), 0,
		"lostPlanets empty when none lost")
}

// TestStatePendingPlanets_Empty verifies an empty slice when all
// planets are terminal.
func TestStatePendingPlanets_Empty(t *testing.T) {
	s := testSovereignWithPlanets(t, []*Planet{
		testDonePlanet(t, 1),
		testLostPlanet(2),
	})
	requireEqual(t, len(s.pendingPlanets()), 0,
		"pendingPlanets empty when all terminal")
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkSaveState measures saveState throughput with 5 planets.
func BenchmarkSaveState(b *testing.B) {
	s := benchmarkSovereign(b)
	s.cfg.Planets = 5
	s.planets = s.initPlanets()
	s.state.Planets = s.planets
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.saveState()
	}
}

// BenchmarkLoadState measures loadState throughput.
func BenchmarkLoadState(b *testing.B) {
	s := benchmarkSovereign(b)
	if err := s.saveState(); err != nil {
		b.Fatalf("BenchmarkLoadState: saveState: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.loadState()
	}
}
