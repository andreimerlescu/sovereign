package commander_test

import (
	"context"
	"errors"
	"io"
	"os"
	"sync"
	"testing"

	"github.com/andreimerlescu/sovereign/pkg/commander"
)

// -- RealCommander -----------------------------------------------------------

func TestRealCommander_TrueCommandSucceeds(t *testing.T) {
	r := &commander.RealCommander{}
	_, _, err := r.Run(context.Background(), "", nil, "true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRealCommander_FalseCommandFails(t *testing.T) {
	r := &commander.RealCommander{}
	_, _, err := r.Run(context.Background(), "", nil, "false")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRealCommander_StdoutCaptured(t *testing.T) {
	r := &commander.RealCommander{}
	stdout, _, err := r.Run(context.Background(), "", nil, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, readErr := io.ReadAll(stdout)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if string(out) != "hello\n" {
		t.Errorf("stdout: got %q want %q", string(out), "hello\n")
	}
}

func TestRealCommander_StderrCaptured(t *testing.T) {
	r := &commander.RealCommander{}
	_, stderr, _ := r.Run(context.Background(), "", nil, "sh", "-c", "echo err >&2")
	out, readErr := io.ReadAll(stderr)
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	if string(out) != "err\n" {
		t.Errorf("stderr: got %q want %q", string(out), "err\n")
	}
}

func TestRealCommander_DirRespected(t *testing.T) {
	r := &commander.RealCommander{}
	dir := t.TempDir()
	stdout, _, err := r.Run(context.Background(), dir, nil, "pwd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, readErr := io.ReadAll(stdout)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if len(out) == 0 {
		t.Error("pwd returned empty output")
	}
}

func TestRealCommander_EnvRespected(t *testing.T) {
	r := &commander.RealCommander{}
	env := append(os.Environ(), "SOVEREIGN_TEST_VAR=testvalue")
	stdout, _, err := r.Run(context.Background(), "", env, "sh", "-c", "echo $SOVEREIGN_TEST_VAR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, readErr := io.ReadAll(stdout)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if string(out) != "testvalue\n" {
		t.Errorf("env: got %q want %q", string(out), "testvalue\n")
	}
}

func TestRealCommander_NilEnvInheritsEnvironment(t *testing.T) {
	r := &commander.RealCommander{}
	_, _, err := r.Run(context.Background(), "", nil, "true")
	if err != nil {
		t.Fatalf("nil env should inherit environment: %v", err)
	}
}

func TestRealCommander_EmptyDirUsesCurrentDir(t *testing.T) {
	r := &commander.RealCommander{}
	_, _, err := r.Run(context.Background(), "", nil, "true")
	if err != nil {
		t.Fatalf("empty dir should use current directory: %v", err)
	}
}

func TestRealCommander_ErrorWrapsCommandName(t *testing.T) {
	r := &commander.RealCommander{}
	_, _, err := r.Run(context.Background(), "", nil, "false")
	if err == nil {
		t.Fatal("expected error")
	}
	if len(err.Error()) < 5 || err.Error()[:5] != "false" {
		t.Errorf("error should be prefixed with command name, got: %v", err)
	}
}

func TestRealCommander_ContextCancelled(t *testing.T) {
	r := &commander.RealCommander{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := r.Run(ctx, "", nil, "sleep", "10")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestRealCommander_ReturnsNonNilReaders(t *testing.T) {
	r := &commander.RealCommander{}
	stdout, stderr, err := r.Run(context.Background(), "", nil, "true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == nil {
		t.Fatal("stdout reader should not be nil")
	}
	if stderr == nil {
		t.Fatal("stderr reader should not be nil")
	}
}

// -- MockCommander -----------------------------------------------------------

func TestMockCommander_DefaultSucceeds(t *testing.T) {
	m := commander.NewMockCommander()
	_, _, err := m.Run(context.Background(), "", nil, "ssh", "-i", "key.pem", "rocky@1.2.3.4", "echo hi")
	if err != nil {
		t.Fatalf("unregistered command should return nil error, got: %v", err)
	}
}

func TestMockCommander_RegisteredErrorReturned(t *testing.T) {
	m := commander.NewMockCommander()
	m.Register("ssh", commander.MockResponse{Err: errors.New("connection refused")})
	_, _, err := m.Run(context.Background(), "", nil, "ssh", "rocky@1.2.3.4")
	if err == nil {
		t.Fatal("expected error from registered handler")
	}
}

func TestMockCommander_RegisteredStdoutReadable(t *testing.T) {
	m := commander.NewMockCommander()
	m.Register("terraform", commander.MockResponse{
		Stdout: `{"instance_id":{"value":"i-abc123"}}`,
	})
	stdout, _, err := m.Run(context.Background(), "/some/dir", []string{"A=B"}, "terraform", "output", "-json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, readErr := io.ReadAll(stdout)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if string(out) != `{"instance_id":{"value":"i-abc123"}}` {
		t.Errorf("stdout: got %q", string(out))
	}
}

func TestMockCommander_RegisteredStderrReadable(t *testing.T) {
	m := commander.NewMockCommander()
	m.Register("terraform", commander.MockResponse{Stderr: "Error: something failed"})
	_, stderr, _ := m.Run(context.Background(), "", nil, "terraform", "apply")
	out, readErr := io.ReadAll(stderr)
	if readErr != nil {
		t.Fatalf("read stderr: %v", readErr)
	}
	if string(out) != "Error: something failed" {
		t.Errorf("stderr: got %q", string(out))
	}
}

func TestMockCommander_CallRecordsDirAndEnv(t *testing.T) {
	m := commander.NewMockCommander()
	env := []string{"AWS_ACCESS_KEY_ID=test"}
	m.Run(context.Background(), "/work/dir", env, "terraform", "init")
	calls := m.Calls("terraform")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Dir != "/work/dir" {
		t.Errorf("Dir: got %q want /work/dir", calls[0].Dir)
	}
	if len(calls[0].Env) != 1 || calls[0].Env[0] != "AWS_ACCESS_KEY_ID=test" {
		t.Errorf("Env: got %v", calls[0].Env)
	}
}

func TestMockCommander_CalledRecordsInvocation(t *testing.T) {
	m := commander.NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "arg1")
	if !m.Called("ssh") {
		t.Fatal("Called should return true after invocation")
	}
	if m.Called("scp") {
		t.Fatal("Called should return false for uninvoked command")
	}
}

func TestMockCommander_CallCountAccurate(t *testing.T) {
	m := commander.NewMockCommander()
	m.Run(context.Background(), "", nil, "scp", "a", "b")
	m.Run(context.Background(), "", nil, "scp", "c", "d")
	m.Run(context.Background(), "", nil, "ssh", "x")
	if m.CallCount("scp") != 2 {
		t.Errorf("CallCount scp: got %d want 2", m.CallCount("scp"))
	}
	if m.CallCount("ssh") != 1 {
		t.Errorf("CallCount ssh: got %d want 1", m.CallCount("ssh"))
	}
	if m.CallCount("terraform") != 0 {
		t.Errorf("CallCount unregistered: got %d want 0", m.CallCount("terraform"))
	}
}

func TestMockCommander_CallsReturnsMatchingOnly(t *testing.T) {
	m := commander.NewMockCommander()
	m.Run(context.Background(), "", nil, "ssh", "planet1")
	m.Run(context.Background(), "", nil, "scp", "planet1")
	m.Run(context.Background(), "", nil, "ssh", "planet2")
	calls := m.Calls("ssh")
	if len(calls) != 2 {
		t.Fatalf("Calls ssh: got %d want 2", len(calls))
	}
	if calls[0].Args[0] != "planet1" {
		t.Errorf("first call arg: got %q want planet1", calls[0].Args[0])
	}
	if calls[1].Args[0] != "planet2" {
		t.Errorf("second call arg: got %q want planet2", calls[1].Args[0])
	}
}

func TestMockCommander_ResetClearsCalls(t *testing.T) {
	m := commander.NewMockCommander()
	m.Register("ssh", commander.MockResponse{Err: errors.New("fail")})
	m.Run(context.Background(), "", nil, "ssh", "arg")
	m.Reset()
	if m.Called("ssh") {
		t.Fatal("Reset should clear calls")
	}
	_, _, err := m.Run(context.Background(), "", nil, "ssh", "arg")
	if err != nil {
		t.Fatalf("Reset should clear handlers, got error: %v", err)
	}
}

func TestMockCommander_ConcurrentSafe(t *testing.T) {
	m := commander.NewMockCommander()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Run(context.Background(), "", nil, "ssh", "arg")
		}()
	}
	wg.Wait()
	if m.CallCount("ssh") != 50 {
		t.Errorf("concurrent calls: got %d want 50", m.CallCount("ssh"))
	}
}

func TestMockCommander_UnregisteredReturnsNonNilReaders(t *testing.T) {
	m := commander.NewMockCommander()
	stdout, stderr, err := m.Run(context.Background(), "", nil, "unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout == nil {
		t.Fatal("stdout should not be nil for unregistered command")
	}
	if stderr == nil {
		t.Fatal("stderr should not be nil for unregistered command")
	}
}

// -- Benchmarks --------------------------------------------------------------

func BenchmarkMockCommanderRun(b *testing.B) {
	m := commander.NewMockCommander()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Run(ctx, "", nil, "ssh", "-i", "key.pem", "rocky@1.2.3.4", "echo hi")
	}
}

func BenchmarkRealCommanderTrue(b *testing.B) {
	r := &commander.RealCommander{}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.Run(ctx, "", nil, "true")
	}
}
