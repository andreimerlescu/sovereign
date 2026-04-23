package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// LedgerEventType identifies what happened at each step of the
// sovereign lifecycle. The ledger is append-only -- events are
// never modified after being written.
type LedgerEventType string

const (
	LedgerEventProvisionStart    LedgerEventType = "provision.start"
	LedgerEventProvisionComplete LedgerEventType = "provision.complete"
	LedgerEventProvisionFailed   LedgerEventType = "provision.failed"
	LedgerEventInitStart         LedgerEventType = "init.start"
	LedgerEventInitComplete      LedgerEventType = "init.complete"
	LedgerEventInitFailed        LedgerEventType = "init.failed"
	LedgerEventApplyStart        LedgerEventType = "apply.start"
	LedgerEventApplyComplete     LedgerEventType = "apply.complete"
	LedgerEventApplyFailed       LedgerEventType = "apply.failed"
	LedgerEventDestroyStart      LedgerEventType = "destroy.start"
	LedgerEventDestroyComplete   LedgerEventType = "destroy.complete"
	LedgerEventDestroyFailed     LedgerEventType = "destroy.failed"
	LedgerEventLemmingsStart     LedgerEventType = "lemmings.start"
	LedgerEventLemmingsComplete  LedgerEventType = "lemmings.complete"
	LedgerEventLemmingsFailed    LedgerEventType = "lemmings.failed"
	LedgerEventCollectStart      LedgerEventType = "collect.start"
	LedgerEventCollectComplete   LedgerEventType = "collect.complete"
	LedgerEventCollectFailed     LedgerEventType = "collect.failed"
	LedgerEventInterrupted       LedgerEventType = "interrupted"
	LedgerEventResumed           LedgerEventType = "resumed"
	LedgerEventValidationFailed  LedgerEventType = "validation.failed"
	LedgerEventDataCorruption    LedgerEventType = "data.corruption"
)

// LedgerEvent is a single entry in the sovereign audit ledger.
// Once written, events are never modified.
type LedgerEvent struct {
	Timestamp  time.Time       `yaml:"Timestamp"`
	PlanetID   int             `yaml:"PlanetID"`
	Provider   string          `yaml:"Provider"`
	Region     string          `yaml:"Region"`
	EventType  LedgerEventType `yaml:"EventType"`
	InstanceID string          `yaml:"InstanceID,omitempty"`
	PublicIPv4 string          `yaml:"PublicIPv4,omitempty"`
	Detail     string          `yaml:"Detail,omitempty"`
	Error      string          `yaml:"Error,omitempty"`
}

// Ledger is the append-only audit log for a single sovereign run.
// All planets write to the same ledger under a mutex.
// Every Append call flushes the entire file to disk immediately.
type Ledger struct {
	mu        sync.Mutex
	path      string
	RunID     string        `yaml:"RunID"`
	StartedAt time.Time     `yaml:"StartedAt"`
	Events    []LedgerEvent `yaml:"Events"`
}

// NewLedger creates a new ledger for a sovereign run. The run
// directory is created if it does not exist. The ledger file is
// written immediately so an empty ledger exists on disk before
// any events are appended.
func NewLedger(sovereignDir string, startedAt time.Time) (*Ledger, error) {
	runID := startedAt.UTC().Format("2006-01-02T15-04-05")
	runDir := filepath.Join(sovereignDir, "runs", runID)
	if err := os.MkdirAll(runDir, 0700); err != nil {
		return nil, fmt.Errorf("ledger: create run dir: %w", err)
	}

	l := &Ledger{
		path:      filepath.Join(runDir, "ledger.yml"),
		RunID:     runID,
		StartedAt: startedAt,
		Events:    []LedgerEvent{},
	}

	if err := l.flush(); err != nil {
		return nil, fmt.Errorf("ledger: initial flush: %w", err)
	}

	return l, nil
}

// LoadLedger reads an existing ledger file from disk for resume.
// Returns an error if the file exists but cannot be read or parsed.
// Returns nil, nil if the file does not exist -- not an error.
func LoadLedger(path string) (*Ledger, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ledger: read %s: %w", path, err)
	}

	var l Ledger
	if err := yaml.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("ledger: parse %s: %w", path, err)
	}

	l.path = path
	return &l, nil
}

// Append adds an event to the ledger and flushes to disk immediately.
// The event timestamp is set here -- callers do not set it.
func (l *Ledger) Append(event LedgerEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	event.Timestamp = time.Now().UTC()
	l.Events = append(l.Events, event)
	return l.flush()
}

// LastEventForPlanet returns the most recent ledger event for a
// given planet ID. Returns nil if no events exist for that planet.
// Used by resume logic to determine where a planet left off.
func (l *Ledger) LastEventForPlanet(planetID int) *LedgerEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.Events) - 1; i >= 0; i-- {
		if l.Events[i].PlanetID == planetID {
			e := l.Events[i]
			return &e
		}
	}
	return nil
}

// PlanetCompleted returns true if the ledger contains a
// LedgerEventCollectComplete event for the given planet ID.
// Used by resume logic to skip planets that already finished.
func (l *Ledger) PlanetCompleted(planetID int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.Events {
		if e.PlanetID == planetID && e.EventType == LedgerEventCollectComplete {
			return true
		}
	}
	return false
}

// PlanetProvisioned returns true if the ledger contains a
// LedgerEventProvisionComplete event for the given planet ID.
// Used by resume logic to skip provisioning for running planets.
func (l *Ledger) PlanetProvisioned(planetID int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.Events {
		if e.PlanetID == planetID && e.EventType == LedgerEventProvisionComplete {
			return true
		}
	}
	return false
}

// Path returns the path to the ledger file on disk.
func (l *Ledger) Path() string {
	return l.path
}

// flush marshals the ledger to YAML and writes it to disk.
// Must be called with l.mu held.
func (l *Ledger) flush() error {
	data, err := yaml.Marshal(l)
	if err != nil {
		return fmt.Errorf("ledger: marshal: %w", err)
	}
	if err := os.WriteFile(l.path, data, 0600); err != nil {
		return fmt.Errorf("ledger: write %s: %w", l.path, err)
	}
	return nil
}
