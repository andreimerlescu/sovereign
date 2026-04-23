package main

import (
	"path/filepath"
	"testing"
	"time"
)

// -- NewLedger ---------------------------------------------------------------

// TestNewLedger_CreatesFile verifies that NewLedger creates the ledger
// file on disk immediately.
func TestNewLedger_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "NewLedger creates file")
	requireTrue(t, l != nil, "NewLedger returns non-nil ledger")

	// file must exist on disk
	if _, statErr := statFile(l.Path()); statErr != nil {
		t.Fatalf("NewLedger: ledger file not found at %s", l.Path())
	}
}

// TestNewLedger_RunIDIsTimestamp verifies that the RunID is derived
// from the startedAt timestamp in the expected format.
func TestNewLedger_RunIDIsTimestamp(t *testing.T) {
	dir := t.TempDir()
	startedAt := time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
	l, err := NewLedger(dir, startedAt)
	requireNoError(t, err, "NewLedger RunID format")
	requireEqual(t, l.RunID, "2026-04-22T14-30-00", "RunID format")
}

// TestNewLedger_EmptyEvents verifies that a newly created ledger
// has an empty events slice.
func TestNewLedger_EmptyEvents(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "NewLedger empty events")
	requireEqual(t, len(l.Events), 0, "NewLedger events empty")
}

// TestNewLedger_CreatesRunDirectory verifies that NewLedger creates
// the runs/<runID> directory structure.
func TestNewLedger_CreatesRunDirectory(t *testing.T) {
	dir := t.TempDir()
	startedAt := time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
	_, err := NewLedger(dir, startedAt)
	requireNoError(t, err, "NewLedger creates run directory")

	runDir := filepath.Join(dir, "runs", "2026-04-22T14-30-00")
	if _, statErr := statFile(runDir); statErr != nil {
		t.Fatalf("NewLedger: run directory not created at %s", runDir)
	}
}

// -- LoadLedger --------------------------------------------------------------

// TestLoadLedger_NonExistentFile verifies that LoadLedger returns
// nil, nil when no ledger file exists.
func TestLoadLedger_NonExistentFile(t *testing.T) {
	l, err := LoadLedger("/nonexistent/path/ledger.yml")
	requireNoError(t, err, "LoadLedger non-existent file")
	requireTrue(t, l == nil, "LoadLedger returns nil for missing file")
}

// TestLoadLedger_RoundTrip verifies that a ledger written by NewLedger
// can be read back by LoadLedger with identical content.
func TestLoadLedger_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	startedAt := time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
	l, err := NewLedger(dir, startedAt)
	requireNoError(t, err, "LoadLedger round trip create")

	// append an event so there's something to verify
	appendErr := l.Append(LedgerEvent{
		PlanetID:  1,
		EventType: LedgerEventProvisionStart,
		Region:    "us-east-1",
	})
	requireNoError(t, appendErr, "LoadLedger round trip append")

	// load it back
	loaded, loadErr := LoadLedger(l.Path())
	requireNoError(t, loadErr, "LoadLedger round trip load")
	requireTrue(t, loaded != nil, "LoadLedger round trip non-nil")
	requireEqual(t, loaded.RunID, l.RunID, "LoadLedger RunID preserved")
	requireEqual(t, len(loaded.Events), 1, "LoadLedger event count preserved")
	requireEqual(t, loaded.Events[0].PlanetID, 1, "LoadLedger event PlanetID preserved")
	requireEqual(t, loaded.Events[0].EventType,
		LedgerEventProvisionStart, "LoadLedger event type preserved")
}

// TestLoadLedger_MalformedFile verifies that a malformed YAML file
// returns a non-nil error.
func TestLoadLedger_MalformedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.yml")
	writeTestFile(t, path, []byte("{ not valid yaml :::"))

	_, err := LoadLedger(path)
	requireError(t, err, "LoadLedger malformed file")
}

// -- Append ------------------------------------------------------------------

// TestAppend_SetsTimestamp verifies that Append sets the event
// timestamp automatically.
func TestAppend_SetsTimestamp(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "Append sets timestamp")

	before := time.Now()
	appendErr := l.Append(LedgerEvent{
		PlanetID:  1,
		EventType: LedgerEventProvisionStart,
	})
	after := time.Now()

	requireNoError(t, appendErr, "Append sets timestamp no error")
	requireTrue(t, !l.Events[0].Timestamp.IsZero(), "Append timestamp non-zero")
	requireTrue(t, !l.Events[0].Timestamp.Before(before),
		"Append timestamp not before call")
	requireTrue(t, !l.Events[0].Timestamp.After(after),
		"Append timestamp not after call")
}

// TestAppend_FlushesToDisk verifies that each Append call writes
// the updated ledger to disk immediately.
func TestAppend_FlushesToDisk(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "Append flushes to disk")

	requireNoError(t, l.Append(LedgerEvent{
		PlanetID:  1,
		EventType: LedgerEventProvisionStart,
	}), "Append flush first event")

	// read the file directly and verify event is present
	loaded, loadErr := LoadLedger(l.Path())
	requireNoError(t, loadErr, "Append flush load")
	requireEqual(t, len(loaded.Events), 1, "Append flush event on disk")
}

// TestAppend_MultipleEvents verifies that multiple Append calls
// accumulate events in order.
func TestAppend_MultipleEvents(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "Append multiple events")

	events := []LedgerEventType{
		LedgerEventProvisionStart,
		LedgerEventInitStart,
		LedgerEventInitComplete,
		LedgerEventApplyStart,
		LedgerEventApplyComplete,
		LedgerEventProvisionComplete,
	}

	for _, et := range events {
		requireNoError(t, l.Append(LedgerEvent{
			PlanetID:  1,
			EventType: et,
		}), "Append multiple events "+string(et))
	}

	requireEqual(t, len(l.Events), len(events), "Append multiple events count")
	for i, et := range events {
		requireEqual(t, l.Events[i].EventType, et,
			"Append multiple events order "+string(et))
	}
}

// TestAppend_ConcurrentSafe verifies that concurrent Append calls
// from multiple goroutines do not panic or corrupt the ledger.
func TestAppend_ConcurrentSafe(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "Append concurrent safe")

	const goroutines = 10
	const eventsEach = 5
	done := make(chan struct{}, goroutines)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			for i := 0; i < eventsEach; i++ {
				_ = l.Append(LedgerEvent{
					PlanetID:  g + 1,
					EventType: LedgerEventProvisionStart,
				})
			}
			done <- struct{}{}
		}()
	}

	for g := 0; g < goroutines; g++ {
		<-done
	}

	requireEqual(t, len(l.Events), goroutines*eventsEach,
		"Append concurrent event count")
}

// -- LastEventForPlanet ------------------------------------------------------

// TestLastEventForPlanet_ReturnsLastEvent verifies that the most
// recent event for a planet is returned.
func TestLastEventForPlanet_ReturnsLastEvent(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "LastEventForPlanet returns last event")

	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventProvisionStart})
	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventInitStart})
	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventApplyComplete})

	last := l.LastEventForPlanet(1)
	requireTrue(t, last != nil, "LastEventForPlanet non-nil")
	requireEqual(t, last.EventType, LedgerEventApplyComplete,
		"LastEventForPlanet returns last event type")
}

// TestLastEventForPlanet_ReturnsNilForUnknownPlanet verifies that
// nil is returned when no events exist for the planet.
func TestLastEventForPlanet_ReturnsNilForUnknownPlanet(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "LastEventForPlanet nil for unknown")

	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventProvisionStart})

	last := l.LastEventForPlanet(99)
	requireTrue(t, last == nil, "LastEventForPlanet nil for unknown planet")
}

// TestLastEventForPlanet_IsolatesByPlanetID verifies that events
// from other planets do not affect the result.
func TestLastEventForPlanet_IsolatesByPlanetID(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "LastEventForPlanet isolates by planet ID")

	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventProvisionStart})
	l.Append(LedgerEvent{PlanetID: 2, EventType: LedgerEventProvisionComplete})
	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventInitStart})

	last := l.LastEventForPlanet(1)
	requireTrue(t, last != nil, "LastEventForPlanet isolates non-nil")
	requireEqual(t, last.EventType, LedgerEventInitStart,
		"LastEventForPlanet isolates correct event")
}

// -- PlanetCompleted ---------------------------------------------------------

// TestPlanetCompleted_TrueWhenCollectComplete verifies that a planet
// with a LedgerEventCollectComplete event is considered complete.
func TestPlanetCompleted_TrueWhenCollectComplete(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "PlanetCompleted true")

	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventProvisionComplete})
	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventLemmingsComplete})
	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventCollectComplete})

	requireTrue(t, l.PlanetCompleted(1), "PlanetCompleted true for collect complete")
}

// TestPlanetCompleted_FalseWithoutCollectComplete verifies that a
// planet without a collect complete event is not considered complete.
func TestPlanetCompleted_FalseWithoutCollectComplete(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "PlanetCompleted false")

	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventProvisionComplete})
	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventLemmingsComplete})

	requireFalse(t, l.PlanetCompleted(1), "PlanetCompleted false without collect complete")
}

// TestPlanetCompleted_FalseForUnknownPlanet verifies that an unknown
// planet ID returns false.
func TestPlanetCompleted_FalseForUnknownPlanet(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "PlanetCompleted false unknown")

	requireFalse(t, l.PlanetCompleted(99), "PlanetCompleted false for unknown planet")
}

// -- PlanetProvisioned -------------------------------------------------------

// TestPlanetProvisioned_TrueWhenProvisionComplete verifies that a
// planet with a provision complete event is considered provisioned.
func TestPlanetProvisioned_TrueWhenProvisionComplete(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "PlanetProvisioned true")

	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventProvisionStart})
	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventProvisionComplete})

	requireTrue(t, l.PlanetProvisioned(1), "PlanetProvisioned true")
}

// TestPlanetProvisioned_FalseWithoutProvisionComplete verifies that
// a planet without a provision complete event is not provisioned.
func TestPlanetProvisioned_FalseWithoutProvisionComplete(t *testing.T) {
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	requireNoError(t, err, "PlanetProvisioned false")

	l.Append(LedgerEvent{PlanetID: 1, EventType: LedgerEventProvisionStart})

	requireFalse(t, l.PlanetProvisioned(1), "PlanetProvisioned false without complete")
}

// -- helpers -----------------------------------------------------------------

func statFile(path string) (interface{}, error) {
	import_os_stat, err := os.Stat(path)
	return import_os_stat, err
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("writeTestFile %s: %v", path, err)
	}
}
