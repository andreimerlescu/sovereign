package commander

import (
	"bytes"
	"context"
	"io"
	"sync"
)

// MockCommander is the test double for Commander. It records every
// invocation and returns pre-configured responses without executing
// real processes.
//
// MockCommander lives in pkg/commander (not a _test.go file) so that
// exeggutor tests, lemmings tests, and main package tests can all
// import it from one place without duplication.
//
// All methods are safe for concurrent use.
type MockCommander struct {
	mu       sync.Mutex
	calls    []MockCall
	handlers map[string]MockResponse
}

// MockCall records a single command invocation including the working
// directory and environment so tests can assert the full subprocess
// context, not just the command name and args.
type MockCall struct {
	Name string
	Dir  string
	Env  []string
	Args []string
}

// MockResponse defines the stdout, stderr, and error returned for a
// registered command name.
type MockResponse struct {
	Stdout string
	Stderr string
	Err    error
}

// NewMockCommander constructs a MockCommander with no registered handlers.
// Unregistered commands return empty readers and nil error by default.
func NewMockCommander() *MockCommander {
	return &MockCommander{
		handlers: make(map[string]MockResponse),
	}
}

// Register sets the response for a command identified by name.
// The key is the command name only -- dir and env are recorded in
// MockCall but are not part of the dispatch key because tests care
// about what ran, not where.
func (m *MockCommander) Register(name string, resp MockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers[name] = resp
}

// Run records the full invocation and returns the registered response
// for name, or empty readers and nil error if no handler is registered.
func (m *MockCommander) Run(_ context.Context, dir string, env []string, name string, args ...string) (io.Reader, io.Reader, error) {
	m.mu.Lock()
	m.calls = append(m.calls, MockCall{
		Name: name,
		Dir:  dir,
		Env:  env,
		Args: args,
	})
	resp, ok := m.handlers[name]
	m.mu.Unlock()

	if !ok {
		return bytes.NewReader(nil), bytes.NewReader(nil), nil
	}
	return bytes.NewBufferString(resp.Stdout),
		bytes.NewBufferString(resp.Stderr),
		resp.Err
}

// Called returns true if the named command was invoked at least once.
func (m *MockCommander) Called(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.calls {
		if c.Name == name {
			return true
		}
	}
	return false
}

// CallCount returns the number of times the named command was invoked.
func (m *MockCommander) CallCount(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, c := range m.calls {
		if c.Name == name {
			count++
		}
	}
	return count
}

// Calls returns all recorded invocations of the named command.
func (m *MockCommander) Calls(name string) []MockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []MockCall
	for _, c := range m.calls {
		if c.Name == name {
			result = append(result, c)
		}
	}
	return result
}

// Reset clears all recorded calls and registered handlers.
func (m *MockCommander) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = nil
	m.handlers = make(map[string]MockResponse)
}
