# commander

> Subprocess execution abstraction for sovereign.
> A Commander interface with a RealCommander for production and a
> MockCommander for tests, so every shell call in sovereign can be
> controlled and inspected without spawning real processes.

---

## What This Package Is

Every subprocess call in sovereign — SSH, SCP, and any other shell
invocation in terrain.go and lemmings.go — goes through the Commander
interface. Nothing calls exec.CommandContext directly outside of this
package and pkg/terraform/exeggutor.

This design means tests can inject MockCommander and fully control
what every subprocess returns without spawning real processes, without
requiring SSH keys, and without requiring a running cloud instance.

MockCommander lives in mock.go rather than a _test.go file so that
the main package tests, lemmings tests, terrain tests, and any other
package that constructs a Sovereign can all import it from one place
without duplication.

---

## Files

    commander.go    Commander interface, RealCommander
    mock.go         MockCommander, MockCall, MockResponse
    commander_test.go   external test and benchmark coverage (package commander_test)
    mock_test.go        internal test and benchmark coverage (package commander)

---

## Dependencies

    standard library
    os/exec    subprocess execution in RealCommander
    bytes      buffer management in MockCommander and RealCommander
    io         MultiWriter in RealCommander, Reader return types

This package never imports any other sovereign package.

---

## Commander Interface

    type Commander interface {
        Run(
            ctx  context.Context,
            dir  string,
            env  []string,
            name string,
            args ...string,
        ) (stdout, stderr io.Reader, err error)
    }

Run executes name with args. dir sets the working directory -- empty
string inherits the current directory. env sets the subprocess
environment -- nil inherits the current process environment.

Returns captured stdout and stderr as io.Reader and any execution
error. Callers may read from the readers after Run returns.

---

## RealCommander

Executes real OS subprocesses via os/exec. Used in production sovereign
runs via NewSovereign.

Stdout and stderr are forwarded live to os.Stdout and os.Stderr via
io.MultiWriter so the operator sees subprocess output in real time.
The same bytes are captured in buffers returned as io.Reader so callers
can inspect output such as terraform JSON responses.

    s.cmd = &commander.RealCommander{}

An empty dir inherits the current working directory. A nil env inherits
the current process environment. Errors are prefixed with the command
name:

    ssh: exit status 255

---

## MockCommander

The test double for Commander. Records every invocation and returns
pre-configured responses without executing real processes.

Construction:

    m := commander.NewMockCommander()

Registering a response for a command:

    m.Register("terraform", commander.MockResponse{
        Stdout: `{"instance_id":{"value":"i-abc123"}}`,
    })

    m.Register("ssh", commander.MockResponse{
        Err: errors.New("connection refused"),
    })

Unregistered commands return empty readers and nil error by default
so tests that do not care about a particular subprocess do not need
to register it.

---

## MockResponse

Defines the stdout string, stderr string, and error returned for a
registered command name. The key is the command name only -- dir and
env are recorded in MockCall but are not part of the dispatch key.

    type MockResponse struct {
        Stdout string
        Stderr string
        Err    error
    }

---

## MockCall

Records a single command invocation. All fields are populated on
every Run call regardless of whether the command is registered.

    type MockCall struct {
        Name string
        Dir  string
        Env  []string
        Args []string
    }

Tests can inspect MockCall to verify not just that a command was
called but that it was called with the correct working directory,
environment, and arguments.

---

## Inspection Methods

After running sovereign code under test, inspect what happened:

    // was ssh called at all?
    m.Called("ssh")

    // how many times was terraform called?
    m.CallCount("terraform")

    // what were the exact arguments for each scp call?
    calls := m.Calls("scp")
    for _, c := range calls {
        fmt.Println(c.Dir, c.Env, c.Args)
    }

    // clear all recorded calls and registered handlers
    m.Reset()

---

## Concurrency

All MockCommander methods are safe for concurrent use. The internal
mutex is held for the minimum duration necessary -- Register and Run
lock only for the map lookup and slice append, not for the duration
of any I/O.

RealCommander is also safe for concurrent use. Each Run call
constructs its own exec.Cmd and captures its own buffers.

---

## Wiring in sovereign

In production NewSovereign sets:

    s.cmd = &commander.RealCommander{}

In tests testSovereign and benchmarkSovereign set:

    s.cmd = commander.NewMockCommander()

Terrain and lemmings code never references either implementation
directly -- they call s.cmd.Run and let the injected implementation
handle the rest.

---

## Design Constraints

Do not call exec.CommandContext directly in terrain.go, lemmings.go,
sovereign.go, or any file outside of commander.go and
pkg/terraform/exeggutor. All subprocess calls go through s.cmd.Run.

Do not import any other sovereign package from this package. Commander
is a leaf dependency. If a proposed change requires importing from
sovereign main or any other sovereign package the dependency belongs
elsewhere.

MockCommander must never be moved to a _test.go file. It lives in
mock.go so that every package that needs a test Sovereign can import
it without duplication or circular dependencies.

---

## Import Path

    import "github.com/andreimerlescu/sovereign/pkg/commander"

---

## Testing

    go test -race ./pkg/commander/...
    go test -bench=. -benchmem ./pkg/commander/...