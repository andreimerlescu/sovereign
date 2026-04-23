# events

> The sovereign event bus.
> A synchronous fan-out dispatcher for planet lifecycle events
> with kind-based filtering and planet-scoped subscription.

---

## What This Package Is

This package is the nervous system of sovereign. Every component
that needs to know what is happening during a sovereign run subscribes
to the event bus. Every component that signals something happened
emits to the event bus. Nothing talks directly to anything else.

This package has zero dependencies outside the Go standard library.
It imports no other sovereign package. It can be compiled and tested
in complete isolation.

---

## Relationship to Lemmings

The lemmings project has its own event bus at
github.com/andreimerlescu/lemmings/pkg/events. Sovereign's event
bus is inspired by that design but is a completely independent
implementation. Sovereign does not import lemmings' event package.

The key differences from lemmings' event bus:

Sovereign's Event struct carries planet domain fields -- PlanetID,
Provider, Region, InstanceID, PublicIPv4 -- instead of lemmings
domain fields like LemmingID, Terrain, Pack.

Sovereign's EventBus adds SubscribeForPlanet as a first-class method
because filtering by planet ID is the most common sovereign use case.
The ledger resume logic, the status reporter, and future dashboard
subscribers all care about specific planets.

Sovereign's EventBus has no EventLog. The ledger on disk is
sovereign's persistent event history. An in-memory replay log is
not needed.

Sovereign's Subscriber contract explicitly permits the ledger
subscriber to flush to disk on every event. This is acceptable
because sovereign lifecycle events are infrequent -- tens of events
per run, not thousands per second. Lemmings' subscriber contract
is stricter because lemmings handles thousands of visit events per
second under swarm load.

---

## Files

    events.go       EventBus, Event, Subscriber, Filter, Tee
    kinds.go        all sovereign EventKind constants
    events_test.go  full test and benchmark coverage

---

## Core Types

### EventKind

A typed string that identifies what happened. All sovereign EventKind
constants are defined in kinds.go. Callers may define additional kinds
as plain string constants -- the EventBus places no restriction on
EventKind values beyond the type.

    type EventKind string

### Event

The unit of communication on the EventBus. Passed by value to every
subscriber. Not all fields are relevant to every EventKind -- zero
values are safe to ignore.

    type Event struct {
        Kind       EventKind
        OccurredAt time.Time

        // Planet identity -- populated on all planet lifecycle events
        PlanetID int
        Provider string
        Region   string

        // Instance detail -- populated after successful provisioning
        InstanceID string
        PublicIPv4 string

        // Human-readable context -- populated on informational events
        Detail string

        // Error -- non-nil on all failed events
        Err error
    }

Subscribers must not retain a reference to Event after their
subscriber function returns. The Err field is an interface and
the underlying error value is shared across all subscribers that
receive the same event.

### Subscriber

A function that receives events from the EventBus.

    type Subscriber func(Event)

Subscribers must not block. The EventBus is synchronous -- every
subscriber runs on the goroutine that called Emit before Emit
returns. A blocking subscriber stalls the sovereign lifecycle
goroutine that emitted the event.

If a subscriber needs to do slow work it must hand off to its own
goroutine or buffered channel internally.

The ledger subscriber is an intentional documented exception. It
flushes to local disk on every event. This is acceptable only
because sovereign lifecycle events are infrequent.

---

## EventBus

A synchronous fan-out event dispatcher. All subscribers receive
every event in registration order. Safe for concurrent use --
Subscribe, SubscribeForPlanet, Emit, and Close may all be called
from multiple goroutines simultaneously.

### Construction

    bus := events.NewEventBus()

### Subscribe

Registers a subscriber to receive all future events. Returns an
unsubscribe function. The slot is set to nil rather than removed
so indices of other subscribers are preserved.

    unsub := bus.Subscribe(func(e events.Event) {
        fmt.Println(e.Kind)
    })
    defer unsub()

### SubscribeForPlanet

Registers a subscriber that only receives events for a specific
planet ID. Optionally filtered to specific EventKinds. Returns
an unsubscribe function identical to Subscribe.

    // all events for planet 3
    unsub := bus.SubscribeForPlanet(3, func(e events.Event) {
        fmt.Printf("planet 3: %s\n", e.Kind)
    })

    // only provision events for planet 3
    unsub := bus.SubscribeForPlanet(3, func(e events.Event) {
        fmt.Printf("planet 3 provision: %s\n", e.Kind)
    }, events.KindProvisionComplete, events.KindProvisionFailed)

### Emit

Dispatches an event to all registered non-nil subscribers.
OccurredAt is set to time.Now() if not already populated.
No-ops if the bus has been closed.

    bus.Emit(events.Event{
        Kind:     events.KindProvisionStart,
        PlanetID: 1,
        Provider: "aws",
        Region:   "us-east-1",
    })

### Close

Marks the bus as closed. Subsequent Emit calls are no-ops.
Subscribers are not notified of closure. Emit any final lifecycle
events before closing if subscribers need to perform cleanup.
Safe to call multiple times.

    bus.Close()

---

## Filter

Returns a Subscriber that only calls fn when the event Kind is in
the allowed set. An empty kinds list produces a subscriber that
never fires.

    bus.Subscribe(events.Filter(handler,
        events.KindProvisionStart,
        events.KindProvisionFailed,
    ))

---

## Tee

Returns a Subscriber that fans an event out to all provided
subscribers. Nil entries are skipped. Useful for attaching multiple
observers to the same event stream without registering them
separately on the bus.

    bus.Subscribe(events.Tee(ledgerHandler, dashboardHandler))

---

## EventKind Constants

All constants are defined in kinds.go grouped by lifecycle phase.

Provision lifecycle:

    KindProvisionStart        planet begins provisioning
    KindProvisionComplete     planet has public IP and instance ID
    KindProvisionFailed       planet failed to provision

Terraform init lifecycle:

    KindInitStart             terraform init begins
    KindInitComplete          terraform init succeeded
    KindInitFailed            terraform init failed

Terraform apply lifecycle:

    KindApplyStart            terraform apply begins
    KindApplyComplete         terraform apply succeeded
    KindApplyFailed           terraform apply failed

Terraform destroy lifecycle:

    KindDestroyStart          terraform destroy begins
    KindDestroyComplete       terraform destroy succeeded
    KindDestroyFailed         terraform destroy failed -- InstanceID
                              carries the ID for manual intervention

Lemmings execution lifecycle:

    KindLemmingsStart         lemmings begins executing on planet
    KindLemmingsComplete      lemmings finished successfully
    KindLemmingsFailed        lemmings failed or timed out

Collection lifecycle:

    KindCollectStart          result collection begins
    KindCollectComplete       results collected, ReportPath populated
    KindCollectFailed         result collection failed

Run lifecycle:

    KindRunResumed            sovereign resumed from existing state
    KindRunInterrupted        planet interrupted mid-lifecycle, recovering

Validation:

    KindValidationFailed      planet or provider failed validation
    KindDataCorruption        inconsistent state detected

---

## Wiring in sovereign

The event bus is constructed in NewSovereign and wired to the ledger
as its first subscriber before any other initialization:

    s.bus = events.NewEventBus()
    s.bus.Subscribe(s.ledger.AsSubscriber())

Emitting events in terrain.go:

    s.bus.Emit(events.Event{
        Kind:     events.KindProvisionStart,
        PlanetID: planet.ID,
        Provider: s.provider.Name(),
        Region:   planet.Region,
    })

Emitting failed events in terrain.go:

    s.bus.Emit(events.Event{
        Kind:     events.KindProvisionFailed,
        PlanetID: planet.ID,
        Provider: s.provider.Name(),
        Region:   planet.Region,
        Err:      err,
    })

Adding a future subscriber without touching emission code:

    s.bus.Subscribe(myDashboardHandler)
    s.bus.Subscribe(myPrometheusExporter)
    s.bus.Subscribe(mySlackNotifier)

---

## Design Constraints

This package must never import any other sovereign package. If a
proposed change requires importing from sovereign main or any other
sovereign package the dependency belongs elsewhere.

This package must never import anything beyond the Go standard
library. It must be compilable and testable in complete isolation.

EventKind values are plain strings. Any caller can define additional
kinds by declaring string constants of type events.EventKind. The
bus places no restriction on kind values.

The synchronous dispatch contract must never be changed to
asynchronous without a clear documented reason. The simplicity of
synchronous dispatch is load-bearing for sovereign's lifecycle
correctness -- it means event ordering is guaranteed and the ledger
always sees events in the order they were emitted.

---

## Import Path

    import "github.com/andreimerlescu/sovereign/pkg/events"

---

## Testing

    go test -race ./pkg/events/...
    go test -bench=. -benchmem ./pkg/events/...​​​​​​​​​​​​​​​​
