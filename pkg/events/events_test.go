package events

import (
	"sync"
	"testing"
	"time"
)

// -- Event -------------------------------------------------------------------

// TestEvent_ZeroValuesAreSafe verifies that an Event with no fields set
// can be emitted and received without panicking.
func TestEvent_ZeroValuesAreSafe(t *testing.T) {
	bus := NewEventBus()
	var received Event
	bus.Subscribe(func(e Event) { received = e })
	bus.Emit(Event{})
	if received.Kind != "" {
		t.Errorf("expected empty Kind, got %q", received.Kind)
	}
}

// TestEvent_AllFieldsPreserved verifies that all Event fields survive
// the bus round-trip.
func TestEvent_AllFieldsPreserved(t *testing.T) {
	bus := NewEventBus()
	var received Event

	bus.Subscribe(func(e Event) { received = e })

	sent := Event{
		Kind:       KindProvisionComplete,
		PlanetID:   3,
		Provider:   "aws",
		Region:     "us-east-1",
		InstanceID: "i-abc123",
		PublicIPv4: "54.1.2.3",
		Detail:     "provisioned successfully",
	}
	bus.Emit(sent)

	if received.Kind != KindProvisionComplete {
		t.Errorf("Kind: got %q want %q", received.Kind, KindProvisionComplete)
	}
	if received.PlanetID != 3 {
		t.Errorf("PlanetID: got %d want 3", received.PlanetID)
	}
	if received.Provider != "aws" {
		t.Errorf("Provider: got %q want aws", received.Provider)
	}
	if received.Region != "us-east-1" {
		t.Errorf("Region: got %q want us-east-1", received.Region)
	}
	if received.InstanceID != "i-abc123" {
		t.Errorf("InstanceID: got %q want i-abc123", received.InstanceID)
	}
	if received.PublicIPv4 != "54.1.2.3" {
		t.Errorf("PublicIPv4: got %q want 54.1.2.3", received.PublicIPv4)
	}
	if received.Detail != "provisioned successfully" {
		t.Errorf("Detail: got %q want provisioned successfully", received.Detail)
	}
}

// TestEvent_ErrFieldPreserved verifies that the Err field survives
// the bus round-trip as a shared interface value.
func TestEvent_ErrFieldPreserved(t *testing.T) {
	bus := NewEventBus()
	var received Event

	bus.Subscribe(func(e Event) { received = e })

	sent := Event{
		Kind:     KindProvisionFailed,
		PlanetID: 1,
		Err:      errForTest("simulated failure"),
	}
	bus.Emit(sent)

	if received.Err == nil {
		t.Fatal("Err should be non-nil after round-trip")
	}
	if received.Err.Error() != "simulated failure" {
		t.Errorf("Err.Error(): got %q want simulated failure", received.Err.Error())
	}
}

// -- NewEventBus -------------------------------------------------------------

// TestNewEventBus_IsOpen verifies that a freshly constructed EventBus
// is open and has no subscribers.
func TestNewEventBus_IsOpen(t *testing.T) {
	bus := NewEventBus()
	if bus == nil {
		t.Fatal("NewEventBus returned nil")
	}
	if bus.closed {
		t.Fatal("new EventBus should not be closed")
	}
	if len(bus.subscribers) != 0 {
		t.Fatalf("new EventBus should have 0 subscribers, got %d",
			len(bus.subscribers))
	}
}

// TestNewEventBus_EmitWithNoSubscribersDoesNotPanic verifies that
// emitting on a bus with no subscribers does not panic.
func TestNewEventBus_EmitWithNoSubscribersDoesNotPanic(t *testing.T) {
	bus := NewEventBus()
	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
}

// -- Subscribe ---------------------------------------------------------------

// TestSubscribe_ReceivesEvents verifies that a registered subscriber
// receives every event emitted after registration.
func TestSubscribe_ReceivesEvents(t *testing.T) {
	bus := NewEventBus()
	var received []Event
	var mu sync.Mutex

	bus.Subscribe(func(e Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
	bus.Emit(Event{Kind: KindApplyComplete, PlanetID: 1})
	bus.Emit(Event{Kind: KindProvisionComplete, PlanetID: 1})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 3 {
		t.Fatalf("expected 3 events, got %d", len(received))
	}
}

// TestSubscribe_MultipleSubscribersAllReceive verifies that all
// registered subscribers receive every event in registration order.
func TestSubscribe_MultipleSubscribersAllReceive(t *testing.T) {
	bus := NewEventBus()
	counts := make([]int, 3)
	var mu sync.Mutex

	for i := 0; i < 3; i++ {
		i := i
		bus.Subscribe(func(e Event) {
			mu.Lock()
			counts[i]++
			mu.Unlock()
		})
	}

	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})

	mu.Lock()
	defer mu.Unlock()
	for i, c := range counts {
		if c != 1 {
			t.Errorf("subscriber %d: expected 1 event, got %d", i, c)
		}
	}
}

// TestSubscribe_Unsubscribe verifies that calling the returned function
// stops the subscriber from receiving events.
func TestSubscribe_Unsubscribe(t *testing.T) {
	bus := NewEventBus()
	var count int
	var mu sync.Mutex

	unsub := bus.Subscribe(func(e Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
	unsub()
	bus.Emit(Event{Kind: KindApplyComplete, PlanetID: 1})

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 event before unsubscribe, got %d", count)
	}
}

// TestSubscribe_UnsubscribeNilsSlotNotRemoves verifies that unsubscribing
// nils the slot rather than reslicing, preserving indices of later
// subscribers.
func TestSubscribe_UnsubscribeNilsSlotNotRemoves(t *testing.T) {
	bus := NewEventBus()
	var secondCount int
	var mu sync.Mutex

	unsub := bus.Subscribe(func(e Event) {})
	bus.Subscribe(func(e Event) {
		mu.Lock()
		secondCount++
		mu.Unlock()
	})

	unsub()
	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})

	mu.Lock()
	defer mu.Unlock()
	if secondCount != 1 {
		t.Fatalf("second subscriber should still receive events after first "+
			"unsubscribes, got %d", secondCount)
	}
}

// -- SubscribeForPlanet ------------------------------------------------------

// TestSubscribeForPlanet_OnlyReceivesMatchingPlanet verifies that the
// subscriber only receives events for the specified planet ID.
func TestSubscribeForPlanet_OnlyReceivesMatchingPlanet(t *testing.T) {
	bus := NewEventBus()
	var received []Event
	var mu sync.Mutex

	bus.SubscribeForPlanet(2, func(e Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 2})
	bus.Emit(Event{Kind: KindApplyComplete, PlanetID: 3})
	bus.Emit(Event{Kind: KindProvisionComplete, PlanetID: 2})

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 events for planet 2, got %d", len(received))
	}
	for _, e := range received {
		if e.PlanetID != 2 {
			t.Errorf("received event for wrong planet: got %d want 2", e.PlanetID)
		}
	}
}

// TestSubscribeForPlanet_WithKindFilter verifies that kind filtering
// works correctly alongside planet filtering.
func TestSubscribeForPlanet_WithKindFilter(t *testing.T) {
	bus := NewEventBus()
	var received []Event
	var mu sync.Mutex

	bus.SubscribeForPlanet(1, func(e Event) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	}, KindProvisionComplete, KindProvisionFailed)

	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})    // filtered by kind
	bus.Emit(Event{Kind: KindProvisionComplete, PlanetID: 1}) // passes
	bus.Emit(Event{Kind: KindProvisionComplete, PlanetID: 2}) // filtered by planet
	bus.Emit(Event{Kind: KindProvisionFailed, PlanetID: 1})   // passes

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
}

// TestSubscribeForPlanet_Unsubscribe verifies that the returned
// unsubscribe function works correctly.
func TestSubscribeForPlanet_Unsubscribe(t *testing.T) {
	bus := NewEventBus()
	var count int
	var mu sync.Mutex

	unsub := bus.SubscribeForPlanet(1, func(e Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
	unsub()
	bus.Emit(Event{Kind: KindApplyComplete, PlanetID: 1})

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 event before unsubscribe, got %d", count)
	}
}

// TestSubscribeForPlanet_PlanetZeroReceivesAll verifies that planet ID
// zero is treated as a valid planet ID and does not accidentally receive
// all events.
func TestSubscribeForPlanet_PlanetZeroReceivesAll(t *testing.T) {
	bus := NewEventBus()
	var count int
	var mu sync.Mutex

	bus.SubscribeForPlanet(0, func(e Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 2})
	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 0})

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatalf("planet 0 subscriber should only receive planet 0 events, got %d",
			count)
	}
}

// -- Emit --------------------------------------------------------------------

// TestEmit_SetsOccurredAt verifies that Emit populates OccurredAt when
// the event has a zero time value.
func TestEmit_SetsOccurredAt(t *testing.T) {
	bus := NewEventBus()
	var received Event

	bus.Subscribe(func(e Event) { received = e })

	before := time.Now()
	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
	after := time.Now()

	if received.OccurredAt.IsZero() {
		t.Fatal("OccurredAt should have been set by Emit")
	}
	if received.OccurredAt.Before(before) || received.OccurredAt.After(after) {
		t.Errorf("OccurredAt %v not within expected range [%v, %v]",
			received.OccurredAt, before, after)
	}
}

// TestEmit_PreservesOccurredAt verifies that Emit does not overwrite
// OccurredAt when already set by the caller.
func TestEmit_PreservesOccurredAt(t *testing.T) {
	bus := NewEventBus()
	var received Event

	bus.Subscribe(func(e Event) { received = e })

	fixed := time.Date(2026, 4, 22, 14, 30, 0, 0, time.UTC)
	bus.Emit(Event{Kind: KindProvisionStart, OccurredAt: fixed})

	if !received.OccurredAt.Equal(fixed) {
		t.Errorf("OccurredAt should be preserved: got %v want %v",
			received.OccurredAt, fixed)
	}
}

// TestEmit_AfterClose verifies that Emit is a no-op after the bus
// has been closed.
func TestEmit_AfterClose(t *testing.T) {
	bus := NewEventBus()
	var count int
	var mu sync.Mutex

	bus.Subscribe(func(e Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
	bus.Close()
	bus.Emit(Event{Kind: KindApplyComplete, PlanetID: 1})
	bus.Emit(Event{Kind: KindProvisionComplete, PlanetID: 1})

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 event before close, got %d", count)
	}
}

// TestEmit_Concurrent verifies that concurrent Emit calls from multiple
// goroutines do not cause data races. Run with -race.
func TestEmit_Concurrent(t *testing.T) {
	bus := NewEventBus()
	var count int64
	var mu sync.Mutex

	bus.Subscribe(func(e Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	const goroutines = 100
	const eventsEach = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			for j := 0; j < eventsEach; j++ {
				bus.Emit(Event{
					Kind:     KindProvisionStart,
					PlanetID: i + 1,
				})
			}
		}()
	}

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	expected := int64(goroutines * eventsEach)
	if count != expected {
		t.Fatalf("expected %d events, got %d", expected, count)
	}
}

// TestSubscribe_Concurrent verifies that concurrent Subscribe and Emit
// calls do not race. Run with -race.
func TestSubscribe_Concurrent(t *testing.T) {
	bus := NewEventBus()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unsub := bus.Subscribe(func(e Event) {})
			bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
			unsub()
		}()
	}

	wg.Wait()
}

// -- Close -------------------------------------------------------------------

// TestClose_Idempotent verifies that calling Close multiple times does
// not panic.
func TestClose_Idempotent(t *testing.T) {
	bus := NewEventBus()
	bus.Close()
	bus.Close()
	bus.Close()
}

// -- Filter ------------------------------------------------------------------

// TestFilter_PassesMatchingKinds verifies that Filter calls the inner
// subscriber only for events whose Kind is in the allowed set.
func TestFilter_PassesMatchingKinds(t *testing.T) {
	var received []EventKind
	var mu sync.Mutex

	fn := Filter(func(e Event) {
		mu.Lock()
		received = append(received, e.Kind)
		mu.Unlock()
	}, KindProvisionStart, KindProvisionComplete)

	fn(Event{Kind: KindProvisionStart})
	fn(Event{Kind: KindApplyComplete}) // not in filter
	fn(Event{Kind: KindProvisionComplete})
	fn(Event{Kind: KindDestroyStart}) // not in filter

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 events through filter, got %d", len(received))
	}
	if received[0] != KindProvisionStart {
		t.Errorf("expected %q, got %q", KindProvisionStart, received[0])
	}
	if received[1] != KindProvisionComplete {
		t.Errorf("expected %q, got %q", KindProvisionComplete, received[1])
	}
}

// TestFilter_BlocksNonMatchingKinds verifies that Filter never calls
// the inner subscriber when no kinds match.
func TestFilter_BlocksNonMatchingKinds(t *testing.T) {
	called := false
	fn := Filter(func(e Event) {
		called = true
	}, KindProvisionStart)

	fn(Event{Kind: KindApplyComplete})
	fn(Event{Kind: KindDestroyStart})
	fn(Event{Kind: KindLemmingsStart})

	if called {
		t.Fatal("Filter called inner subscriber for non-matching kind")
	}
}

// TestFilter_EmptyKindList verifies that a Filter with no allowed kinds
// never passes any event through.
func TestFilter_EmptyKindList(t *testing.T) {
	called := false
	fn := Filter(func(e Event) {
		called = true
	})

	fn(Event{Kind: KindProvisionStart})
	fn(Event{Kind: KindApplyComplete})

	if called {
		t.Fatal("Filter with empty kind list should never call inner subscriber")
	}
}

// TestFilter_Integration verifies Filter and Subscribe work correctly
// together end-to-end on a live bus.
func TestFilter_Integration(t *testing.T) {
	bus := NewEventBus()
	var received []EventKind
	var mu sync.Mutex

	bus.Subscribe(Filter(func(e Event) {
		mu.Lock()
		received = append(received, e.Kind)
		mu.Unlock()
	}, KindProvisionStart, KindProvisionFailed))

	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
	bus.Emit(Event{Kind: KindApplyComplete, PlanetID: 1}) // filtered
	bus.Emit(Event{Kind: KindProvisionFailed, PlanetID: 1})
	bus.Emit(Event{Kind: KindDestroyStart, PlanetID: 1}) // filtered

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("expected 2 events through filter on live bus, got %d",
			len(received))
	}
}

// -- Tee ---------------------------------------------------------------------

// TestTee_FansToAllSubscribers verifies that Tee delivers every event
// to all provided subscribers.
func TestTee_FansToAllSubscribers(t *testing.T) {
	counts := make([]int, 3)
	var mu sync.Mutex

	subs := make([]Subscriber, 3)
	for i := 0; i < 3; i++ {
		i := i
		subs[i] = func(e Event) {
			mu.Lock()
			counts[i]++
			mu.Unlock()
		}
	}

	fn := Tee(subs...)
	fn(Event{Kind: KindProvisionStart, PlanetID: 1})
	fn(Event{Kind: KindApplyComplete, PlanetID: 1})

	mu.Lock()
	defer mu.Unlock()
	for i, c := range counts {
		if c != 2 {
			t.Errorf("subscriber %d: expected 2 events, got %d", i, c)
		}
	}
}

// TestTee_NilSubscribersSkipped verifies that Tee skips nil subscribers
// without panicking.
func TestTee_NilSubscribersSkipped(t *testing.T) {
	var count int
	fn := Tee(
		nil,
		func(e Event) { count++ },
		nil,
	)
	fn(Event{Kind: KindProvisionStart, PlanetID: 1})
	if count != 1 {
		t.Fatalf("expected 1 call from non-nil subscriber, got %d", count)
	}
}

// TestTee_Empty verifies that Tee with no subscribers does not panic.
func TestTee_Empty(t *testing.T) {
	fn := Tee()
	fn(Event{Kind: KindProvisionStart, PlanetID: 1})
}

// TestTee_Integration verifies Tee and Subscribe work correctly
// together end-to-end on a live bus.
func TestTee_Integration(t *testing.T) {
	bus := NewEventBus()
	var aCount, bCount int
	var mu sync.Mutex

	a := func(e Event) { mu.Lock(); aCount++; mu.Unlock() }
	b := func(e Event) { mu.Lock(); bCount++; mu.Unlock() }

	bus.Subscribe(Tee(a, b))
	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
	bus.Emit(Event{Kind: KindProvisionComplete, PlanetID: 1})

	mu.Lock()
	defer mu.Unlock()
	if aCount != 2 {
		t.Errorf("subscriber a: expected 2, got %d", aCount)
	}
	if bCount != 2 {
		t.Errorf("subscriber b: expected 2, got %d", bCount)
	}
}

// -- EventKind constants -----------------------------------------------------

// TestEventKindConstants_AreUniqueStrings verifies that all EventKind
// constants have unique string values. Duplicate constants would cause
// subscribers using kind-based filtering to receive unexpected events.
func TestEventKindConstants_AreUniqueStrings(t *testing.T) {
	all := []EventKind{
		KindProvisionStart, KindProvisionComplete, KindProvisionFailed,
		KindInitStart, KindInitComplete, KindInitFailed,
		KindApplyStart, KindApplyComplete, KindApplyFailed,
		KindDestroyStart, KindDestroyComplete, KindDestroyFailed,
		KindLemmingsStart, KindLemmingsComplete, KindLemmingsFailed,
		KindCollectStart, KindCollectComplete, KindCollectFailed,
		KindRunResumed, KindRunInterrupted,
		KindValidationFailed, KindDataCorruption,
	}

	seen := make(map[EventKind]bool, len(all))
	for _, k := range all {
		if seen[k] {
			t.Errorf("duplicate EventKind constant: %q", k)
		}
		seen[k] = true
	}
}

// TestEventKindConstants_AreNonEmpty verifies that no EventKind constant
// is an empty string.
func TestEventKindConstants_AreNonEmpty(t *testing.T) {
	all := []EventKind{
		KindProvisionStart, KindProvisionComplete, KindProvisionFailed,
		KindInitStart, KindInitComplete, KindInitFailed,
		KindApplyStart, KindApplyComplete, KindApplyFailed,
		KindDestroyStart, KindDestroyComplete, KindDestroyFailed,
		KindLemmingsStart, KindLemmingsComplete, KindLemmingsFailed,
		KindCollectStart, KindCollectComplete, KindCollectFailed,
		KindRunResumed, KindRunInterrupted,
		KindValidationFailed, KindDataCorruption,
	}

	for _, k := range all {
		if k == "" {
			t.Errorf("EventKind constant has empty string value")
		}
	}
}

// -- Benchmarks --------------------------------------------------------------

// BenchmarkEmit_OneSubscriber measures Emit throughput with a single
// subscriber. Establishes the baseline cost of a dispatch.
func BenchmarkEmit_OneSubscriber(b *testing.B) {
	bus := NewEventBus()
	bus.Subscribe(func(e Event) {})
	evt := Event{
		Kind:     KindProvisionStart,
		PlanetID: 1,
		Provider: "aws",
		Region:   "us-east-1",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Emit(evt)
	}
}

// BenchmarkEmit_TenSubscribers measures Emit throughput with ten
// subscribers -- representative of a sovereign run with ledger,
// status reporter, and several planet-specific subscribers active.
func BenchmarkEmit_TenSubscribers(b *testing.B) {
	bus := NewEventBus()
	for i := 0; i < 10; i++ {
		bus.Subscribe(func(e Event) {})
	}
	evt := Event{
		Kind:     KindProvisionComplete,
		PlanetID: 1,
		Provider: "aws",
		Region:   "us-east-1",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Emit(evt)
	}
}

// BenchmarkEmit_Concurrent measures Emit throughput under concurrent
// planet provisioning -- five goroutines emitting simultaneously
// representing five planets provisioning in parallel.
func BenchmarkEmit_Concurrent(b *testing.B) {
	bus := NewEventBus()
	bus.Subscribe(func(e Event) {})
	evt := Event{Kind: KindProvisionStart, PlanetID: 1}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bus.Emit(evt)
		}
	})
}

// BenchmarkSubscribeForPlanet_Filtering measures the overhead of
// planet-ID filtering relative to unfiltered subscription.
func BenchmarkSubscribeForPlanet_Filtering(b *testing.B) {
	bus := NewEventBus()
	bus.SubscribeForPlanet(1, func(e Event) {})
	bus.SubscribeForPlanet(2, func(e Event) {})
	bus.SubscribeForPlanet(3, func(e Event) {})

	// Only planet 2 events pass through to its subscriber
	evt := Event{Kind: KindProvisionStart, PlanetID: 2}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Emit(evt)
	}
}

// BenchmarkFilter measures the overhead of kind-based filtering.
func BenchmarkFilter(b *testing.B) {
	var count int
	fn := Filter(func(e Event) { count++ },
		KindProvisionComplete, KindApplyComplete, KindCollectComplete)

	evt := Event{Kind: KindProvisionComplete, PlanetID: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fn(evt)
	}
}

// -- helpers -----------------------------------------------------------------

// errForTest is a minimal error implementation for test use.
type errForTest string

func (e errForTest) Error() string { return string(e) }
