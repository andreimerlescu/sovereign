# ledger

> The append-only audit log for a sovereign run.
> Every planet lifecycle event is written to disk immediately
> and never modified after writing.

---

## What This Package Is

The ledger is sovereign's persistent audit trail. It records every
lifecycle event for every planet in a single YAML file per run.
It is the source of truth for resume logic -- when sovereign restarts
after an interruption it reads the ledger to determine what each
planet has already accomplished without re-provisioning work that
already succeeded.

The ledger is a subscriber on sovereign's event bus. It does not
need to be called directly from terrain or lemmings code. Emitting
an event to the bus automatically appends it to the ledger.

---

## Files

    ledger.go       Ledger, Entry, NewLedger, LoadLedger, Append, AsSubscriber
    ledger_test.go  full test and benchmark coverage

---

## Dependencies

    github.com/andreimerlescu/sovereign/pkg/events   EventKind, Event, Subscriber
    gopkg.in/yaml.v3                                 YAML serialization
    standard library

This package never imports sovereign main or any other sovereign
package beyond pkg/events.

---

## Core Types

### Entry

The disk-serialized form of a sovereign lifecycle event. Written
once and never modified. All fields map directly from events.Event.
The Err field from events.Event is serialized as a string so the
YAML file is human-readable without Go type information.

    type Entry struct {
        Timestamp  time.Time
        Kind       events.EventKind
        PlanetID   int
        Provider   string
        Region     string
        InstanceID string
        PublicIPv4 string
        Detail     string
        Error      string
    }

### Ledger

The in-memory and on-disk ledger for a sovereign run. Thread-safe.
All planets write to the same ledger instance under a mutex.

---

## Construction

NewLedger creates a fresh ledger for a new sovereign run:

    l, err := ledger.NewLedger(sovereignDir, startedAt)

sovereignDir is typically ~/.sovereignd. The ledger is written to:

    ~/.sovereignd/runs/<2006-01-02T15-04-05>/ledger.yml

The RunID is derived from the startedAt timestamp in UTC. The
ledger file is created immediately on construction so an empty
ledger exists on disk before any events are appended.

LoadLedger reads an existing ledger for resume:

    l, err := ledger.LoadLedger(path)

Returns nil, nil if the file does not exist -- not an error.
Returns an error if the file exists but cannot be read or parsed.

---

## Wiring to the Event Bus

The ledger is wired to sovereign's event bus as a subscriber in
NewSovereign. Every event emitted on the bus is automatically
appended to the ledger:

    s.ledger, err = ledger.NewLedger(sovereignDir, s.startedAt)
    s.bus.Subscribe(s.ledger.AsSubscriber())

After this point no code in sovereign needs to call ledger.Append
directly. Emit to the bus instead:

    s.bus.Emit(events.Event{
        Kind:     events.KindProvisionStart,
        PlanetID: planet.ID,
        Provider: s.provider.Name(),
        Region:   planet.Region,
    })

---

## Resume Logic

On restart sovereign calls these methods to route each planet:

    PlanetCompleted(planetID)

        Returns true if a KindCollectComplete entry exists for the
        planet. Planet has finished everything -- skip entirely.

    PlanetProvisioned(planetID)

        Returns true if a KindProvisionComplete entry exists for the
        planet. Planet has a running instance -- skip provisioning
        and resume from lemmings execution.

    LastEntryForPlanet(planetID)

        Returns the most recent entry for the planet. Used to
        determine exactly where in the lifecycle the planet stopped
        when neither PlanetCompleted nor PlanetProvisioned match.
        Returns nil if no entries exist for the planet.

---

## Flush Behavior

Every Append call flushes the entire ledger file to disk immediately.
This is intentional. The ledger is the crash recovery mechanism --
if sovereign exits unexpectedly between two events, every event that
was appended before the crash is guaranteed to be on disk.

The performance cost of flush-on-append is acceptable because
sovereign lifecycle events are infrequent. A typical five-planet run
produces fewer than one hundred ledger entries over thirty minutes.
This is not a high-frequency write path.

---

## On-Disk Format

The ledger file is a human-readable YAML file:

    RunID: 2026-04-22T14-30-00
    StartedAt: 2026-04-22T14:30:00Z
    Entries:
      - Timestamp: 2026-04-22T14:30:01Z
        Kind: provision.start
        PlanetID: 1
        Provider: aws
        Region: us-east-1
      - Timestamp: 2026-04-22T14:38:22Z
        Kind: provision.complete
        PlanetID: 1
        Provider: aws
        Region: us-east-1
        InstanceID: i-0abc123def456789a
        PublicIPv4: 54.1.2.3

Optional fields (Provider, Region, InstanceID, PublicIPv4, Detail,
Error) are omitted from the YAML when empty via omitempty tags.

---

## AsSubscriber Error Handling

If Append fails -- due to a disk full condition or permissions
error -- AsSubscriber logs the error to stderr and continues.
A ledger write failure is serious but does not abort the sovereign
run. The planet lifecycle continues and the operator is notified
via stderr output.

---

## Import Path

    import "github.com/andreimerlescu/sovereign/pkg/ledger"

---

## Testing

    go test -race ./pkg/ledger/...
    go test -bench=. -benchmem ./pkg/ledger/...
