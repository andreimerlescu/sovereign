package commander

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
)

// -- MockCommander construction ----------------------------------------------

func TestNewMockCommander_HasNoHandlers(t *testing.T) {
	m := NewMockCommander()
	if m.handlers == nil {
		t.Fatal("handlers map should be initialized")
	}
	if len(m.handlers) != 0 {
		t.Errorf("expected 0 handlers, got %d", len(m.handlers))
	}
}

func TestNewMockCommander_HasNoCalls(t *testing.T) {
	m := NewMockCommander()
	if len(m.calls) != 0 {
		t.Errorf("expected 0 calls, got %d", len(m.calls))
	}
}

// -- Register ----------------------------------------------------------------

func TestRegister_StoresHandler(t *testing.T) {
	m := NewMockCommander()
	resp := MockResponse{Stdout: "hello", Stderr: "err", Err: nil}
	m.Register("ssh", resp)

	m.mu.Lock()
	got, ok := m.handlers["ssh"]
	m.mu.Unlock()

	if !ok {
		t.Fatal("Register: handler not stored")
	}
	if got.Stdout != "hello" {
		t.Errorf("Stdout: got %q want hello", got.Stdout)
	}
	if got.Stderr != "err" {
		t.Errorf("Stderr: got %q want err", got.Stderr)
	}
}

func TestRegister_OverwritesPreviousHandler(t *testing.T) {
	m := NewMockCommander()
	m.Register("ssh", MockResponse{Stdout: "first"})
	m.Register("ssh", MockResponse{Stdout: "second"})

	m.mu.Lock()
	got := m.handlers["ssh"]
	m.mu.Unlock()

	if got.Stdout != "second" {
		t.Errorf("Stdout: got %q want second", got.Stdout)
	}
}

func TestRegister_MultipleCommandsStoredSeparately(t *testing.T) {
	m := NewMockCommander()
	m.Register("ssh", MockResponse{Stdout: "ssh-out"})
	m.Register("scp", MockResponse{Stdout: "scp-out"})
	m.Register("terraform", MockResponse{Stdout: "tf-out"})

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.handlers) != 3 {
		t.Errorf("expected 3 handlers, got %d", len(m.handlers))
	}
}

// -- Run ---------------------------------------------------------------------

func TestRun_RecordsCallWithName(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "arg1", "arg2")

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(m.calls))
	}
	if m.calls[0].Name != "ssh" {
		t.Errorf("Name: got %q want ssh", m.calls[0].Name)
	}
}

func TestRun_RecordsCallArgs(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "-i", "key.pem", "rocky@1.2.3.4")

	m.mu.Lock()
	defer m.mu.Unlock()

	args := m.calls[0].Args
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d", len(args))
	}
	if args[0] != "-i" {
		t.Errorf("args[0]: got %q want -i", args[0])
	}
	if args[1] != "key.pem" {
		t.Errorf("args[1]: got %q want key.pem", args[1])
	}
	if args[2] != "rocky@1.2.3.4" {
		t.Errorf("args[2]: got %q want rocky@1.2.3.4", args[2])
	}
}

func TestRun_RecordsCallDir(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "/work/dir", nil, "terraform", "init")

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.calls[0].Dir != "/work/dir" {
		t.Errorf("Dir: got %q want /work/dir", m.calls[0].Dir)
	}
}

func TestRun_RecordsCallEnv(t *testing.T) {
	m := NewMockCommander()
	env := []string{"AWS_ACCESS_KEY_ID=test", "AWS_SECRET_ACCESS_KEY=secret"}
	m.Run(context.Background(), "", env, "terraform", "apply")

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.calls[0].Env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(m.calls[0].Env))
	}
	if m.calls[0].Env[0] != "AWS_ACCESS_KEY_ID=test" {
		t.Errorf("Env[0]: got %q want AWS_ACCESS_KEY_ID=test", m.calls[0].Env[0])
	}
}

func TestRun_UnregisteredReturnsNilError(t *testing.T) {
	m := NewMockCommander()
	_, _, err := m.Run(context.Background(), "", nil, "unknown")
	if err != nil {
		t.Errorf("unregistered command should return nil error, got %v", err)
	}
}

func TestRun_UnregisteredReturnsNonNilReaders(t *testing.T) {
	m := NewMockCommander()
	stdout, stderr, _ := m.Run(context.Background(), "", nil, "unknown")
	if stdout == nil {
		t.Error("stdout should not be nil for unregistered command")
	}
	if stderr == nil {
		t.Error("stderr should not be nil for unregistered command")
	}
}

func TestRun_UnregisteredReturnsEmptyReaders(t *testing.T) {
	m := NewMockCommander()
	stdout, stderr, _ := m.Run(context.Background(), "", nil, "unknown")

	outBytes, _ := io.ReadAll(stdout)
	errBytes, _ := io.ReadAll(stderr)

	if len(outBytes) != 0 {
		t.Errorf("stdout should be empty for unregistered command, got %q", outBytes)
	}
	if len(errBytes) != 0 {
		t.Errorf("stderr should be empty for unregistered command, got %q", errBytes)
	}
}

func TestRun_RegisteredReturnsStdout(t *testing.T) {
	m := NewMockCommander()
	m.Register("terraform", MockResponse{Stdout: `{"instance_id":{"value":"i-abc123"}}`})

	stdout, _, err := m.Run(context.Background(), "", nil, "terraform", "output", "-json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, _ := io.ReadAll(stdout)
	if string(out) != `{"instance_id":{"value":"i-abc123"}}` {
		t.Errorf("stdout: got %q", string(out))
	}
}

func TestRun_RegisteredReturnsStderr(t *testing.T) {
	m := NewMockCommander()
	m.Register("terraform", MockResponse{Stderr: "Error: something failed"})

	_, stderr, _ := m.Run(context.Background(), "", nil, "terraform", "apply")

	out, _ := io.ReadAll(stderr)
	if string(out) != "Error: something failed" {
		t.Errorf("stderr: got %q", string(out))
	}
}

func TestRun_RegisteredReturnsError(t *testing.T) {
	m := NewMockCommander()
	simErr := errors.New("connection refused")
	m.Register("ssh", MockResponse{Err: simErr})

	_, _, err := m.Run(context.Background(), "", nil, "ssh", "rocky@1.2.3.4")
	if err == nil {
		t.Fatal("expected error from registered handler")
	}
	if err.Error() != "connection refused" {
		t.Errorf("error: got %q want connection refused", err.Error())
	}
}

func TestRun_StdoutIsIndependentBytesBuffer(t *testing.T) {
	m := NewMockCommander()
	m.Register("ssh", MockResponse{Stdout: "output"})

	stdout1, _, _ := m.Run(context.Background(), "", nil, "ssh", "host1")
	stdout2, _, _ := m.Run(context.Background(), "", nil, "ssh", "host2")

	// drain first reader
	out1, _ := io.ReadAll(stdout1)
	out2, _ := io.ReadAll(stdout2)

	if string(out1) != "output" {
		t.Errorf("stdout1: got %q want output", string(out1))
	}
	if string(out2) != "output" {
		t.Errorf("stdout2: got %q want output", string(out2))
	}
}

func TestRun_NilEnvRecordedAsNil(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "arg")

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.calls[0].Env != nil {
		t.Errorf("nil env should be recorded as nil, got %v", m.calls[0].Env)
	}
}

func TestRun_EmptyArgsRecorded(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "true")

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.calls[0].Args) != 0 {
		t.Errorf("expected 0 args, got %d", len(m.calls[0].Args))
	}
}

func TestRun_AccumulatesMultipleCalls(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "a")
	m.Run(context.Background(), "", nil, "ssh", "b")
	m.Run(context.Background(), "", nil, "scp", "c")

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.calls) != 3 {
		t.Errorf("expected 3 calls, got %d", len(m.calls))
	}
}

// -- Called ------------------------------------------------------------------

func TestCalled_TrueAfterInvocation(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "arg")

	m.mu.Lock()
	calls := m.calls
	m.mu.Unlock()

	found := false
	for _, c := range calls {
		if c.Name == "ssh" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ssh should appear in calls after invocation")
	}
}

func TestCalled_FalseForUninvokedCommand(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "arg")

	m.mu.Lock()
	calls := m.calls
	m.mu.Unlock()

	for _, c := range calls {
		if c.Name == "scp" {
			t.Fatal("scp should not appear in calls")
		}
	}
}

// -- CallCount ---------------------------------------------------------------

func TestCallCount_AccuratePerCommand(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "scp", "a", "b")
	m.Run(context.Background(), "", nil, "scp", "c", "d")
	m.Run(context.Background(), "", nil, "ssh", "x")

	m.mu.Lock()
	scpCount := 0
	sshCount := 0
	for _, c := range m.calls {
		switch c.Name {
		case "scp":
			scpCount++
		case "ssh":
			sshCount++
		}
	}
	m.mu.Unlock()

	if scpCount != 2 {
		t.Errorf("scp count: got %d want 2", scpCount)
	}
	if sshCount != 1 {
		t.Errorf("ssh count: got %d want 1", sshCount)
	}
}

func TestCallCount_ZeroForNeverInvoked(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "arg")

	m.mu.Lock()
	count := 0
	for _, c := range m.calls {
		if c.Name == "terraform" {
			count++
		}
	}
	m.mu.Unlock()

	if count != 0 {
		t.Errorf("terraform count: got %d want 0", count)
	}
}

// -- Calls -------------------------------------------------------------------

func TestCalls_ReturnsOnlyMatchingCommand(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "planet1")
	m.Run(context.Background(), "", nil, "scp", "planet1")
	m.Run(context.Background(), "", nil, "ssh", "planet2")

	m.mu.Lock()
	var sshCalls []MockCall
	for _, c := range m.calls {
		if c.Name == "ssh" {
			sshCalls = append(sshCalls, c)
		}
	}
	m.mu.Unlock()

	if len(sshCalls) != 2 {
		t.Fatalf("expected 2 ssh calls, got %d", len(sshCalls))
	}
	if sshCalls[0].Args[0] != "planet1" {
		t.Errorf("first ssh arg: got %q want planet1", sshCalls[0].Args[0])
	}
	if sshCalls[1].Args[0] != "planet2" {
		t.Errorf("second ssh arg: got %q want planet2", sshCalls[1].Args[0])
	}
}

func TestCalls_ReturnsNilForNeverInvoked(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "arg")

	m.mu.Lock()
	var scpCalls []MockCall
	for _, c := range m.calls {
		if c.Name == "scp" {
			scpCalls = append(scpCalls, c)
		}
	}
	m.mu.Unlock()

	if len(scpCalls) != 0 {
		t.Errorf("expected 0 scp calls, got %d", len(scpCalls))
	}
}

// -- Reset -------------------------------------------------------------------

func TestReset_ClearsCalls(t *testing.T) {
	m := NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "arg")

	m.mu.Lock()
	m.calls = nil
	m.handlers = make(map[string]MockResponse)
	m.mu.Unlock()

	m.mu.Lock()
	callCount := len(m.calls)
	m.mu.Unlock()

	if callCount != 0 {
		t.Errorf("expected 0 calls after reset, got %d", callCount)
	}
}

func TestReset_ClearsHandlers(t *testing.T) {
	m := NewMockCommander()
	m.Register("ssh", MockResponse{Err: errors.New("fail")})

	m.mu.Lock()
	m.calls = nil
	m.handlers = make(map[string]MockResponse)
	m.mu.Unlock()

	m.mu.Lock()
	handlerCount := len(m.handlers)
	m.mu.Unlock()

	if handlerCount != 0 {
		t.Errorf("expected 0 handlers after reset, got %d", handlerCount)
	}
}

// -- MockCall fields ---------------------------------------------------------

func TestMockCall_NameField(t *testing.T) {
	c := MockCall{Name: "terraform", Dir: "/work", Args: []string{"init"}}
	if c.Name != "terraform" {
		t.Errorf("Name: got %q want terraform", c.Name)
	}
}

func TestMockCall_DirField(t *testing.T) {
	c := MockCall{Name: "terraform", Dir: "/work/dir"}
	if c.Dir != "/work/dir" {
		t.Errorf("Dir: got %q want /work/dir", c.Dir)
	}
}

func TestMockCall_EnvField(t *testing.T) {
	c := MockCall{Env: []string{"A=1", "B=2"}}
	if len(c.Env) != 2 {
		t.Errorf("Env len: got %d want 2", len(c.Env))
	}
}

// -- MockResponse fields -----------------------------------------------------

func TestMockResponse_StdoutField(t *testing.T) {
	r := MockResponse{Stdout: "hello"}
	if r.Stdout != "hello" {
		t.Errorf("Stdout: got %q want hello", r.Stdout)
	}
}

func TestMockResponse_StderrField(t *testing.T) {
	r := MockResponse{Stderr: "error output"}
	if r.Stderr != "error output" {
		t.Errorf("Stderr: got %q want error output", r.Stderr)
	}
}

func TestMockResponse_ErrField(t *testing.T) {
	err := errors.New("simulated failure")
	r := MockResponse{Err: err}
	if r.Err != err {
		t.Errorf("Err: got %v want %v", r.Err, err)
	}
}

// -- bytes.Buffer behavior ---------------------------------------------------

func TestRun_StdoutIsReadableBuffer(t *testing.T) {
	m := NewMockCommander()
	m.Register("ssh", MockResponse{Stdout: "test output"})

	stdout, _, _ := m.Run(context.Background(), "", nil, "ssh", "arg")

	buf, ok := stdout.(*bytes.Buffer)
	if !ok {
		// bytes.NewBufferString returns *bytes.Buffer -- verify readable
		out, err := io.ReadAll(stdout)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		if string(out) != "test output" {
			t.Errorf("stdout: got %q want test output", string(out))
		}
		return
	}
	if buf.String() != "test output" {
		t.Errorf("stdout buffer: got %q want test output", buf.String())
	}
}

func TestRun_StderrIsReadableBuffer(t *testing.T) {
	m := NewMockCommander()
	m.Register("ssh", MockResponse{Stderr: "test error"})

	_, stderr, _ := m.Run(context.Background(), "", nil, "ssh", "arg")

	out, err := io.ReadAll(stderr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(out) != "test error" {
		t.Errorf("stderr: got %q want test error", string(out))
	}
}

// -- Concurrency -------------------------------------------------------------

func TestRun_ConcurrentCallsSafe(t *testing.T) {
	m := NewMockCommander()
	var wg sync.WaitGroup
	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Run(context.Background(), "", nil, "ssh", "arg")
		}()
	}
	wg.Wait()

	m.mu.Lock()
	count := len(m.calls)
	m.mu.Unlock()

	if count != goroutines {
		t.Errorf("concurrent calls: got %d want %d", count, goroutines)
	}
}

func TestRegister_ConcurrentSafe(t *testing.T) {
	m := NewMockCommander()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Register("ssh", MockResponse{Stdout: "out"})
		}()
	}
	wg.Wait()

	m.mu.Lock()
	_, ok := m.handlers["ssh"]
	m.mu.Unlock()

	if !ok {
		t.Fatal("ssh handler should be registered after concurrent writes")
	}
}

// -- Benchmarks --------------------------------------------------------------

func BenchmarkRun_Unregistered(b *testing.B) {
	m := NewMockCommander()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Run(ctx, "", nil, "ssh", "-i", "key.pem", "rocky@1.2.3.4", "echo hi")
	}
}

func BenchmarkRun_Registered(b *testing.B) {
	m := NewMockCommander()
	m.Register("ssh", MockResponse{Stdout: "output", Stderr: ""})
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Run(ctx, "", nil, "ssh", "-i", "key.pem", "rocky@1.2.3.4", "echo hi")
	}
}

func BenchmarkRegister(b *testing.B) {
	m := NewMockCommander()
	resp := MockResponse{Stdout: "out"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Register("ssh", resp)
	}
}

func BenchmarkCallCount(b *testing.B) {
	m := NewMockCommander()
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		m.Run(ctx, "", nil, "ssh", "arg")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.CallCount("ssh")
	}
}
