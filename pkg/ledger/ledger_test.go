package ledger

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/andreimerlescu/sovereign/pkg/events"
)

// -- helpers -----------------------------------------------------------------

func newTestLedger(t *testing.T) *Ledger {
	t.Helper()
	dir := t.TempDir()
	l, err := NewLedger(dir, time.Now())
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	return l
}

func provisionEvent(planetID int) events.Event {
	return events.Event{
		Kind:     events.KindProvisionStart,
		PlanetID: planetID,
		Provider: "aws",
		Region:   "us-east-1",
	}
}

func completeEvent(planetID int) events.Event {
	return events.Event{
		Kind:       events.KindProvisionComplete,
		PlanetID:   planetID,
		Provider:   "aws",
		Region:     "us-east-1",
		InstanceID: "i-abc123",
		PublicIPv4: "54.1.2.3",
	}
}

func collectCompleteEvent(planetID int) events.Event {
	return events.Event{
		Kind:     events.KindCollectComplete,
		PlanetID: planetID,
	}
}

// -- NewLedger ---------------------------------------------------------------

// TestNewLedger_CreatesFile verifies that NewLedger creates the ledger
// file on disk immediately.
func TestNewLedger_CreatesFile(t *testing.T) {
	l := newTestLedger(t)
	if _, err := os.Stat(l.Path()); err != nil {
		t.Fatalf("ledger file not found at %s", l.Path())
	}
}

// TestNewLedger_RunIDIsTimestamp verifies that the RunID is derived
// from the startedAt timestamp in the expected format.
func TestNewLedger_RunIDIsTimestamp(t *testing.T) {
	dir := t.TempDir()
	startedAt := time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
	l, err := NewLedger(dir, startedAt)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	if l.RunID() != "2026-04-22T14-30-00" {
		t.Errorf("RunID: got %q want 2026-04-22T14-30-00", l.RunID())
	}
}

// TestNewLedger_EmptyEntries verifies that a newly created ledger
// has an empty entries slice.
func TestNewLedger_EmptyEntries(t *testing.T) {
	l := newTestLedger(t)
	if len(l.entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(l.entries))
	}
}

// TestNewLedger_CreatesRunDirectory verifies that NewLedger creates
// the runs/<runID> directory structure.
func TestNewLedger_CreatesRunDirectory(t *testing.T) {
	dir := t.TempDir()
	startedAt := time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
	_, err := NewLedger(dir, startedAt)
	if err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	runDir := filepath.Join(dir, "runs", "2026-04-22T14-30-00")
	if _, err := os.Stat(runDir); err != nil {
		t.Fatalf("run directory not created at %s", runDir)
	}
}

// -- LoadLedger --------------------------------------------------------------

// TestLoadLedger_NonExistentFile verifies that LoadLedger returns
// nil, nil when no ledger file exists.
func TestLoadLedger_NonExistentFile(t *testing.T) {
	l, err := LoadLedger("/nonexistent/path/ledger.yml")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if l != nil {
		t.Fatal("expected nil ledger for missing file")
	}
}

// TestLoadLedger_RoundTrip verifies that a ledger written by NewLedger
// can be read back by LoadLedger with identical content.
func TestLoadLedger_RoundTrip(t *testing.T) {
	l := newTestLedger(t)

	if err := l.Append(provisionEvent(1)); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := l.Append(completeEvent(1)); err != nil {
		t.Fatalf("Append: %v", err)
	}

	loaded, err := LoadLedger(l.Path())
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadLedger returned nil")
	}
	if loaded.RunID() != l.RunID() {
		t.Errorf("RunID: got %q want %q", loaded.RunID(), l.RunID())
	}
	if len(loaded.entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded.entries))
	}
	if loaded.entries[0].Kind != events.KindProvisionStart {
		t.Errorf("first entry Kind: got %q want %q",
			loaded.entries[0].Kind, events.KindProvisionStart)
	}
	if loaded.entries[1].Kind != events.KindProvisionComplete {
		t.Errorf("second entry Kind: got %q want %q",
			loaded.entries[1].Kind, events.KindProvisionComplete)
	}
}

// TestLoadLedger_MalformedFile verifies that a malformed YAML file
// returns a non-nil error.
func TestLoadLedger_MalformedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.yml")
	if err := os.WriteFile(path, []byte("{ not valid yaml :::"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := LoadLedger(path)
	if err == nil {
		t.Fatal("expected error for malformed file, got nil")
	}
}

// -- Append ------------------------------------------------------------------

// TestAppend_SetsTimestamp verifies that Append sets the entry
// Timestamp from OccurredAt.
func TestAppend_SetsTimestamp(t *testing.T) {
	l := newTestLedger(t)

	fixed := time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
	e := events.Event{
		Kind:       events.KindProvisionStart,
		PlanetID:   1,
		OccurredAt: fixed,
	}
	if err := l.Append(e); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if !l.entries[0].Timestamp.Equal(fixed) {
		t.Errorf("Timestamp: got %v want %v", l.entries[0].Timestamp, fixed)
	}
}

// TestAppend_ZeroOccurredAtUsesNow verifies that when OccurredAt is
// zero the entry Timestamp is set to approximately now.
func TestAppend_ZeroOccurredAtUsesNow(t *testing.T) {
	l := newTestLedger(t)

	before := time.Now()
	if err := l.Append(events.Event{
		Kind:     events.KindProvisionStart,
		PlanetID: 1,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	after := time.Now()

	ts := l.entries[0].Timestamp
	if ts.IsZero() {
		t.Fatal("Timestamp should not be zero")
	}
	if ts.Before(before) || ts.After(after) {
		t.Errorf("Timestamp %v not in expected range [%v, %v]", ts, before, after)
	}
}

// TestAppend_FlushesToDisk verifies that each Append call writes
// the updated ledger to disk immediately.
func TestAppend_FlushesToDisk(t *testing.T) {
	l := newTestLedger(t)

	if err := l.Append(provisionEvent(1)); err != nil {
		t.Fatalf("Append: %v", err)
	}

	loaded, err := LoadLedger(l.Path())
	if err != nil {
		t.Fatalf("LoadLedger: %v", err)
	}
	if len(loaded.entries) != 1 {
		t.Fatalf("expected 1 entry on disk, got %d", len(loaded.entries))
	}
}

// TestAppend_ErrorFieldSerialized verifies that the Err field from
// events.Event is serialized as a string in the Entry.
func TestAppend_ErrorFieldSerialized(t *testing.T) {
	l := newTestLedger(t)

	if err := l.Append(events.Event{
		Kind:     events.KindProvisionFailed,
		PlanetID: 1,
		Err:      testErr("simulated failure"),
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	if l.entries[0].Error != "simulated failure" {
		t.Errorf("Error: got %q want simulated failure", l.entries[0].Error)
	}
}

// TestAppend_AllFieldsPreserved verifies that all Event fields are
// correctly stored in the Entry.
func TestAppend_AllFieldsPreserved(t *testing.T) {
	l := newTestLedger(t)

	e := events.Event{
		Kind:       events.KindProvisionComplete,
		PlanetID:   5,
		Provider:   "ovh",
		Region:     "GRA11",
		InstanceID: "abc-def-123",
		PublicIPv4: "1.2.3.4",
		Detail:     "test detail",
	}
	if err := l.Append(e); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entry := l.entries[0]
	if entry.Kind != events.KindProvisionComplete {
		t.Errorf("Kind: got %q want %q", entry.Kind, events.KindProvisionComplete)
	}
	if entry.PlanetID != 5 {
		t.Errorf("PlanetID: got %d want 5", entry.PlanetID)
	}
	if entry.Provider != "ovh" {
		t.Errorf("Provider: got %q want ovh", entry.Provider)
	}
	if entry.Region != "GRA11" {
		t.Errorf("Region: got %q want GRA11", entry.Region)
	}
	if entry.InstanceID != "abc-def-123" {
		t.Errorf("InstanceID: got %q want abc-def-123", entry.InstanceID)
	}
	if entry.PublicIPv4 != "1.2.3.4" {
		t.Errorf("PublicIPv4: got %q want 1.2.3.4", entry.PublicIPv4)
	}
	if entry.Detail != "test detail" {
		t.Errorf("Detail: got %q want test detail", entry.Detail)
	}
}

// TestAppend_MultipleEntries verifies that multiple Append calls
// accumulate entries in order.
func TestAppend_MultipleEntries(t *testing.T) {
	l := newTestLedger(t)

	kinds := []events.EventKind{
		events.KindProvisionStart,
		events.KindInitStart,
		events.KindInitComplete,
		events.KindApplyStart,
		events.KindApplyComplete,
		events.KindProvisionComplete,
	}

	for _, k := range kinds {
		if err := l.Append(events.Event{
			Kind:     k,
			PlanetID: 1,
		}); err != nil {
			t.Fatalf("Append %s: %v", k, err)
		}
	}

	if len(l.entries) != len(kinds) {
		t.Fatalf("expected %d entries, got %d", len(kinds), len(l.entries))
	}
	for i, k := range kinds {
		if l.entries[i].Kind != k {
			t.Errorf("entry[%d].Kind: got %q want %q", i, l.entries[i].Kind, k)
		}
	}
}

// TestAppend_ConcurrentSafe verifies that concurrent Append calls
// from multiple goroutines do not panic or corrupt the ledger.
func TestAppend_ConcurrentSafe(t *testing.T) {
	l := newTestLedger(t)

	const goroutines = 10
	const entriesEach = 5
	done := make(chan struct{}, goroutines)

	for g := 0; g < goroutines; g++ {
		g := g
		go func() {
			for i := 0; i < entriesEach; i++ {
				_ = l.Append(events.Event{
					Kind:     events.KindProvisionStart,
					PlanetID: g + 1,
				})
			}
			done <- struct{}{}
		}()
	}

	for g := 0; g < goroutines; g++ {
		<-done
	}

	if len(l.entries) != goroutines*entriesEach {
		t.Errorf("expected %d entries, got %d",
			goroutines*entriesEach, len(l.entries))
	}
}

// -- AsSubscriber ------------------------------------------------------------

// TestAsSubscriber_AppendsCalled verifies that the subscriber returned
// by AsSubscriber appends events to the ledger.
func TestAsSubscriber_AppendsCalled(t *testing.T) {
	l := newTestLedger(t)
	sub := l.AsSubscriber()

	sub(provisionEvent(1))
	sub(completeEvent(1))

	if len(l.entries) != 2 {
		t.Fatalf("expected 2 entries via AsSubscriber, got %d", len(l.entries))
	}
}

// TestAsSubscriber_IntegratesWithEventBus verifies that AsSubscriber
// works correctly when registered on an events.EventBus.
func TestAsSubscriber_IntegratesWithEventBus(t *testing.T) {
	l := newTestLedger(t)

	bus := newTestBus()
	bus.subscribe(l.AsSubscriber())

	bus.emit(provisionEvent(1))
	bus.emit(completeEvent(1))
	bus.emit(collectCompleteEvent(1))

	if len(l.entries) != 3 {
		t.Fatalf("expected 3 entries from bus, got %d", len(l.entries))
	}
	if l.entries[2].Kind != events.KindCollectComplete {
		t.Errorf("last entry Kind: got %q want %q",
			l.entries[2].Kind, events.KindCollectComplete)
	}
}

// -- LastEntryForPlanet ------------------------------------------------------

// TestLastEntryForPlanet_ReturnsLastEntry verifies that the most recent
// entry for a planet is returned.
func TestLastEntryForPlanet_ReturnsLastEntry(t *testing.T) {
	l := newTestLedger(t)

	l.Append(events.Event{PlanetID: 1, Kind: events.KindProvisionStart})
	l.Append(events.Event{PlanetID: 1, Kind: events.KindInitStart})
	l.Append(events.Event{PlanetID: 1, Kind: events.KindApplyComplete})

	last := l.LastEntryForPlanet(1)
	if last == nil {
		t.Fatal("expected non-nil entry")
	}
	if last.Kind != events.KindApplyComplete {
		t.Errorf("Kind: got %q want %q", last.Kind, events.KindApplyComplete)
	}
}

// TestLastEntryForPlanet_ReturnsNilForUnknownPlanet verifies nil is
// returned when no entries exist for the planet.
func TestLastEntryForPlanet_ReturnsNilForUnknownPlanet(t *testing.T) {
	l := newTestLedger(t)
	l.Append(events.Event{PlanetID: 1, Kind: events.KindProvisionStart})

	if l.LastEntryForPlanet(99) != nil {
		t.Fatal("expected nil for unknown planet")
	}
}

// TestLastEntryForPlanet_IsolatesByPlanetID verifies that events from
// other planets do not affect the result.
func TestLastEntryForPlanet_IsolatesByPlanetID(t *testing.T) {
	l := newTestLedger(t)

	l.Append(events.Event{PlanetID: 1, Kind: events.KindProvisionStart})
	l.Append(events.Event{PlanetID: 2, Kind: events.KindProvisionComplete})
	l.Append(events.Event{PlanetID: 1, Kind: events.KindInitStart})

	last := l.LastEntryForPlanet(1)
	if last == nil {
		t.Fatal("expected non-nil entry for planet 1")
	}
	if last.Kind != events.KindInitStart {
		t.Errorf("Kind: got %q want %q", last.Kind, events.KindInitStart)
	}
}

// -- PlanetCompleted ---------------------------------------------------------

// TestPlanetCompleted_TrueWhenCollectComplete verifies that a planet
// with a KindCollectComplete entry is considered complete.
func TestPlanetCompleted_TrueWhenCollectComplete(t *testing.T) {
	l := newTestLedger(t)

	l.Append(events.Event{PlanetID: 1, Kind: events.KindProvisionComplete})
	l.Append(events.Event{PlanetID: 1, Kind: events.KindLemmingsComplete})
	l.Append(events.Event{PlanetID: 1, Kind: events.KindCollectComplete})

	if !l.PlanetCompleted(1) {
		t.Fatal("expected planet 1 to be completed")
	}
}

// TestPlanetCompleted_FalseWithoutCollectComplete verifies that a
// planet without a collect complete entry is not complete.
func TestPlanetCompleted_FalseWithoutCollectComplete(t *testing.T) {
	l := newTestLedger(t)

	l.Append(events.Event{PlanetID: 1, Kind: events.KindProvisionComplete})
	l.Append(events.Event{PlanetID: 1, Kind: events.KindLemmingsComplete})

	if l.PlanetCompleted(1) {
		t.Fatal("expected planet 1 to not be completed without collect complete")
	}
}

// TestPlanetCompleted_FalseForUnknownPlanet verifies false for an
// unknown planet ID.
func TestPlanetCompleted_FalseForUnknownPlanet(t *testing.T) {
	l := newTestLedger(t)
	if l.PlanetCompleted(99) {
		t.Fatal("expected false for unknown planet")
	}
}

// TestPlanetCompleted_IsolatesByPlanetID verifies that a completed
// planet does not affect the result for other planets.
func TestPlanetCompleted_IsolatesByPlanetID(t *testing.T) {
	l := newTestLedger(t)

	l.Append(events.Event{PlanetID: 1, Kind: events.KindCollectComplete})

	if l.PlanetCompleted(2) {
		t.Fatal("planet 2 should not be completed because planet 1 completed")
	}
}

// -- PlanetProvisioned -------------------------------------------------------

// TestPlanetProvisioned_TrueWhenProvisionComplete verifies that a
// planet with a provision complete entry is considered provisioned.
func TestPlanetProvisioned_TrueWhenProvisionComplete(t *testing.T) {
	l := newTestLedger(t)

	l.Append(events.Event{PlanetID: 1, Kind: events.KindProvisionStart})
	l.Append(events.Event{PlanetID: 1, Kind: events.KindProvisionComplete})

	if !l.PlanetProvisioned(1) {
		t.Fatal("expected planet 1 to be provisioned")
	}
}

// TestPlanetProvisioned_FalseWithoutProvisionComplete verifies that
// a planet without a provision complete entry is not provisioned.
func TestPlanetProvisioned_FalseWithoutProvisionComplete(t *testing.T) {
	l := newTestLedger(t)

	l.Append(events.Event{PlanetID: 1, Kind: events.KindProvisionStart})

	if l.PlanetProvisioned(1) {
		t.Fatal("expected planet 1 to not be provisioned without complete entry")
	}
}

// TestPlanetProvisioned_FalseForUnknownPlanet verifies false for an
// unknown planet ID.
func TestPlanetProvisioned_FalseForUnknownPlanet(t *testing.T) {
	l := newTestLedger(t)
	if l.PlanetProvisioned(99) {
		t.Fatal("expected false for unknown planet")
	}
}

// -- entryFromEvent ----------------------------------------------------------

// TestEntryFromEvent_MapsAllFields verifies that entryFromEvent
// correctly maps all events.Event fields to Entry fields.
func TestEntryFromEvent_MapsAllFields(t *testing.T) {
	fixed := time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
	e := events.Event{
		Kind:       events.KindProvisionComplete,
		OccurredAt: fixed,
		PlanetID:   3,
		Provider:   "do",
		Region:     "nyc1",
		InstanceID: "i-123",
		PublicIPv4: "1.2.3.4",
		Detail:     "done",
		Err:        testErr("oops"),
	}

	entry := entryFromEvent(e)

	if entry.Timestamp != fixed {
		t.Errorf("Timestamp: got %v want %v", entry.Timestamp, fixed)
	}
	if entry.Kind != events.KindProvisionComplete {
		t.Errorf("Kind: got %q want %q", entry.Kind, events.KindProvisionComplete)
	}
	if entry.PlanetID != 3 {
		t.Errorf("PlanetID: got %d want 3", entry.PlanetID)
	}
	if entry.Provider != "do" {
		t.Errorf("Provider: got %q want do", entry.Provider)
	}
	if entry.Region != "nyc1" {
		t.Errorf("Region: got %q want​​​​​​​​​​​​​​​​
