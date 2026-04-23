package events

import (
	"sync"
	"time"
)

// Event is the unit of communication on the sovereign EventBus.
//
// Fields are populated depending on EventKind -- not all fields are
// relevant to every event. Zero values are safe to ignore.
//
// Subscribers must not retain a reference to Event after their
// subscriber function returns. Events are value types passed by copy
// -- but the Err field is an interface and the underlying error value
// is shared.
type Event struct {
	Kind       EventKind
	OccurredAt time.Time

	// Planet identity -- populated on all planet lifecycle events.
	PlanetID int
	Provider string
	Region   string

	// Instance detail -- populated after successful provisioning.
	InstanceID string
	PublicIPv4 string

	// Human-readable context -- populated on informational events.
	Detail string

	// Error -- non-nil on all failed events.
	Err error
}

// Subscriber is a function that receives events from the EventBus.
//
// Subscribers must not block. Sovereign's EventBus is synchronous --
// every subscriber runs on the goroutine that called Emit before Emit
// returns. A blocking subscriber stalls the entire sovereign lifecycle
// goroutine that emitted the event.
//
// If a subscriber needs to do slow work -- writing to a remote API,
// sending a notification -- it must hand off to its own goroutine or
// buffered channel internally.
//
// The ledger subscriber is an intentional exception: it flushes to
// local disk on every event. This is acceptable because sovereign
// lifecycle events are infrequent -- tens of events per run, not
// thousands per second.
type Subscriber func(Event)

// EventBus is a synchronous fan-out event dispatcher for the sovereign
// lifecycle.
//
// All subscribers receive every event in registration order. Subscribers
// are called on the goroutine that calls Emit. The bus is safe for
// concurrent use -- Subscribe, SubscribeForPlanet, Emit, and Close may
// all be called from multiple goroutines simultaneously.
//
// Usage:
//
//	bus := NewEventBus()
//	unsub := bus.Subscribe(func(e Event) {
//	    fmt.Println(e.Kind)
//	})
//	defer unsub()
//	bus.Emit(Event{Kind: KindProvisionStart, PlanetID: 1})
//
// Closing the bus does not drain in-flight Emit calls. Ensure all
// emitters have stopped before calling Close if ordering guarantees
// are required.
type EventBus struct {
	mu          sync.RWMutex
	subscribers []Subscriber
	closed      bool
}

// NewEventBus constructs an empty EventBus ready to accept subscribers.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make([]Subscriber, 0, 8),
	}
}

// Subscribe registers a subscriber to receive all future events.
//
// Returns an unsubscribe function -- call it to stop receiving events.
// The slot is set to nil rather than removed, preserving the indices
// of other subscribers registered after this one.
//
// Safe to call concurrently with Emit and other Subscribe calls.
func (b *EventBus) Subscribe(fn Subscriber) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.subscribers = append(b.subscribers, fn)
	idx := len(b.subscribers) - 1

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if idx < len(b.subscribers) {
			b.subscribers[idx] = nil
		}
	}
}

// SubscribeForPlanet registers a subscriber that only receives events
// for the given planet ID. It is equivalent to:
//
//	bus.Subscribe(Filter(fn, kinds...))
//
// but additionally filters on PlanetID so the subscriber never sees
// events from other planets.
//
// If no kinds are provided the subscriber receives all event kinds for
// the given planet.
//
// Returns an unsubscribe function identical to Subscribe.
func (b *EventBus) SubscribeForPlanet(planetID int, fn Subscriber, kinds ...EventKind) func() {
	var filtered Subscriber
	if len(kinds) == 0 {
		filtered = func(e Event) {
			if e.PlanetID == planetID {
				fn(e)
			}
		}
	} else {
		allowed := make(map[EventKind]bool, len(kinds))
		for _, k := range kinds {
			allowed[k] = true
		}
		filtered = func(e Event) {
			if e.PlanetID == planetID && allowed[e.Kind] {
				fn(e)
			}
		}
	}
	return b.Subscribe(filtered)
}

// Emit dispatches an event to all registered non-nil subscribers.
//
// OccurredAt is set to time.Now() if not already populated by the caller.
// Nil subscriber slots (unsubscribed) are skipped without cost.
// No-ops if the bus has been closed.
func (b *EventBus) Emit(e Event) {
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now()
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return
	}

	for _, fn := range b.subscribers {
		if fn != nil {
			fn(e)
		}
	}
}

// Close marks the bus as closed. Subsequent Emit calls are no-ops.
//
// Subscribers are not notified of closure. Emit any final lifecycle
// events before closing if subscribers need to perform cleanup.
//
// Safe to call multiple times -- subsequent calls are no-ops.
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
}

// Filter returns a Subscriber that only calls fn when the event Kind
// is in the allowed set.
//
// Usage:
//
//	bus.Subscribe(Filter(handler, KindProvisionStart, KindProvisionFailed))
//
// An empty kinds list produces a subscriber that never fires.
func Filter(fn Subscriber, kinds ...EventKind) Subscriber {
	allowed := make(map[EventKind]bool, len(kinds))
	for _, k := range kinds {
		allowed[k] = true
	}
	return func(e Event) {
		if allowed[e.Kind] {
			fn(e)
		}
	}
}

// Tee returns a Subscriber that fans an event out to all provided
// subscribers. Nil entries in the list are skipped.
//
// Useful for attaching multiple observers to the same event stream
// without registering them separately on the bus.
//
// Usage:
//
//	bus.Subscribe(Tee(ledgerHandler, dashboardHandler))
func Tee(subscribers ...Subscriber) Subscriber {
	return func(e Event) {
		for _, fn := range subscribers {
			if fn != nil {
				fn(e)
			}
		}
	}
}
