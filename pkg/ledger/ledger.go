package ledger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/andreimerlescu/sovereign/pkg/events"
)

// Entry is the disk-serialized form of a sovereign lifecycle event.
// It is written to the ledger file on every Append call and is never
// modified after writing.
//
// All fields map directly from events.Event. Timestamp maps from
// events.Event.OccurredAt and is promoted to a top-level field for
// YAML readability.
type Entry struct {
	Timestamp  time.Time        `yaml:"Timestamp"`
	Kind       events.EventKind `yaml:"Kind"`
	PlanetID   int              `yaml:"PlanetID"`
	Provider   string           `yaml:"Provider,omitempty"`
	Region     string           `yaml:"Region,omitempty"`
	InstanceID string           `yaml:"InstanceID,omitempty"`
	PublicIPv4 string           `yaml:"PublicIPv4,omitempty"`
	Detail     string           `yaml:"Detail,omitempty"`
	Error      string           `yaml:"Error,omitempty"`
}

// entryFromEvent converts an events.Event into a ledger Entry.
// The Timestamp is set from OccurredAt. If OccurredAt is zero
// time.Now().UTC() is used so the entry always has a valid timestamp.
func entryFromEvent(e events.Event) Entry {
	ts := e.OccurredAt
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	entry := Entry{
		Timestamp:  ts,
		Kind:       e.Kind,
		PlanetID:   e.PlanetID,
		Provider:   e.Provider,
		Region:     e.Region,
		InstanceID: e.InstanceID,
		PublicIPv4: e.PublicIPv4,
		Detail:     e.Detail,
	}
	if e.Err != nil {
		entry.Error = e.Err.Error()
	}
	return entry
}

// ledgerFile is the on-disk structure of the ledger YAML file.
// Separated from Ledger so the in-memory mutex is not serialized.
type ledgerFile struct {
	RunID     string    `yaml:"RunID"`
	StartedAt time.Time `yaml:"StartedAt"`
	Entries   []Entry   `yaml:"Entries"`
}

// Ledger is the append-only audit log for a single sovereign run.
// All planets write to the same ledger under a mutex. Every Append
// call flushes the entire file to disk immediately.
//
// The ledger is the source of truth for resume logic. On restart
// sovereign reads the ledger to determine what each planet has
// already accomplished and routes accordingly without re-provisioning
// work that already succeeded.
type Ledger struct {
	mu        sync.Mutex
	path      string
	runID     string
	startedAt time.Time
	entries   []Entry
}

// NewLedger creates a new ledger for a sovereign run. The run directory
// is created if it does not exist. The ledger file is written immediately
// so an empty ledger exists on disk before any events are appended.
//
// sovereignDir is the root sovereign directory, typically ~/.sovereignd.
// The ledger is written to sovereignDir/runs/<runID>/ledger.yml.
func NewLedger(sovereignDir string, startedAt time.Time) (*Ledger, error) {
	runID := startedAt.UTC().Format("2006-01-02T15-04-05")
	runDir := filepath.Join(sovereignDir, "runs", runID)
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return nil, fmt.Errorf("ledger: create run dir: %w", err)
	}

	l := &Ledger{
		path:      filepath.Join(runDir, "ledger.yml"),
		runID:     runID,
		startedAt: startedAt,
		entries:   []Entry{},
	}

	if err := l.flush(); err != nil {
		return nil, fmt.Errorf("ledger: initial flush: %w", err)
	}

	return l, nil
}

// LoadLedger reads an existing ledger file from disk for resume.
// Returns nil, nil if the file does not exist -- not an error.
// Returns an error if the file exists but cannot be read or parsed.
func LoadLedger(path string) (*Ledger, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ledger: read %s: %w", path, err)
	}

	var f ledgerFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("ledger: parse %s: %w", path, err)
	}

	return &Ledger{
		path:      path,
		runID:     f.RunID,
		startedAt: f.StartedAt,
		entries:   f.Entries,
	}, nil
}

// Append converts an events.Event into an Entry and writes it to the
// ledger. Flushes to disk immediately after appending.
//
// Append is called by the subscriber returned from AsSubscriber and
// should not need to be called directly from sovereign code. Emit to
// the event bus instead.
func (l *Ledger) Append(e events.Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entryFromEvent(e))
	return l.flush()
}

// AsSubscriber returns an events.Subscriber that appends every received
// event to the ledger. Register this on the event bus in NewSovereign:
//
//	s.bus.Subscribe(s.ledger.AsSubscriber())
//
// Errors from Append are logged to stderr but do not panic -- a ledger
// write failure is serious but should not abort the sovereign run.
func (l *Ledger) AsSubscriber() events.Subscriber {
	return func(e events.Event) {
		if err := l.Append(e); err != nil {
			fmt.Fprintf(os.Stderr,
				"  sovereign: ledger: failed to append event %s for planet %d: %v\n",
				e.Kind, e.PlanetID, err)
		}
	}
}

// LastEntryForPlanet returns the most recent ledger entry for the given
// planet ID. Returns nil if no entries exist for that planet.
//
// Used by resume logic to determine where a planet left off after an
// interrupted sovereign run.
func (l *Ledger) LastEntryForPlanet(planetID int) *Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.entries) - 1; i >= 0; i-- {
		if l.entries[i].PlanetID == planetID {
			e := l.entries[i]
			return &e
		}
	}
	return nil
}

// PlanetCompleted returns true if the ledger contains a
// KindCollectComplete entry for the given planet ID.
//
// Used by resume logic to skip planets that already finished
// result collection in a previous run.
func (l *Ledger) PlanetCompleted(planetID int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if e.PlanetID == planetID && e.Kind == events.KindCollectComplete {
			return true
		}
	}
	return false
}

// PlanetProvisioned returns true if the ledger contains a
// KindProvisionComplete entry for the given planet ID.
//
// Used by resume logic to skip provisioning for planets that already
// have a running instance from a previous run.
func (l *Ledger) PlanetProvisioned(planetID int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if e.PlanetID == planetID && e.Kind == events.KindProvisionComplete {
			return true
		}
	}
	return false
}

// Path returns the path to the ledger file on disk.
func (l *Ledger) Path() string {
	return l.path
}

// RunID returns the run identifier derived from the startedAt timestamp.
func (l *Ledger) RunID() string {
	return l.runID
}

// flush marshals the ledger to YAML and writes it to disk.
// Must be called with l.mu held.
func (l *Ledger) flush() error {
	f := ledgerFile{
		RunID:     l.runID,
		StartedAt: l.startedAt,
		Entries:   l.entries,
	}
	data, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("ledger: marshal: %w", err)
	}
	if err := os.WriteFile(l.path, data, 0600); err != nil {
		return fmt.Errorf("ledger: write %s: %w", l.path, err)
	}
	return nil
}
