package commander

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Commander abstracts all subprocess execution in sovereign.
// Every shell call -- SSH, SCP, terraform -- goes through Commander
// so tests can inject MockCommander and fully control subprocess
// behavior without spawning real processes.
//
// Run executes name with args in dir using env as the process environment.
// An empty dir means the current working directory is inherited.
// A nil env means the current process environment is inherited.
//
// Stdout and stderr are returned as io.Reader. For RealCommander these
// are backed by bytes.Buffer and written simultaneously to os.Stdout
// and os.Stderr via io.MultiWriter so the operator sees live output
// while callers can also inspect the captured content.
type Commander interface {
	Run(ctx context.Context, dir string, env []string, name string, args ...string) (stdout, stderr io.Reader, err error)
}

// RealCommander executes real OS subprocesses via os/exec.
//
// Stdout and stderr are forwarded live to os.Stdout and os.Stderr via
// io.MultiWriter so the operator sees subprocess output in real time.
// The same bytes are captured in buffers returned as io.Reader so
// callers can inspect output such as terraform JSON responses.
type RealCommander struct{}

// Run executes name with args. dir sets the working directory; empty
// string inherits the current directory. env sets the subprocess
// environment; nil inherits the current environment.
//
// Returns captured stdout and stderr as io.Reader and any execution
// error prefixed with the command name.
func (r *RealCommander) Run(ctx context.Context, dir string, env []string, name string, args ...string) (io.Reader, io.Reader, error) {
	var outBuf, errBuf bytes.Buffer

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = io.MultiWriter(os.Stdout, &outBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &errBuf)

	if dir != "" {
		cmd.Dir = dir
	}
	if env != nil {
		cmd.Env = env
	}

	if err := cmd.Run(); err != nil {
		return &outBuf, &errBuf, fmt.Errorf("%s: %w", name, err)
	}

	return &outBuf, &errBuf, nil
}
