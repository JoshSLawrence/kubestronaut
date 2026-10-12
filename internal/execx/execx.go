// Package execx is the single place external processes are started, so that
// tests can fake docker, kind and kubectl without the binaries installed.
package execx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// killGrace is how long Wait may block on output pipes after the process was
// killed, in case a grandchild still holds them open.
const killGrace = 5 * time.Second

// Cmd describes one process invocation.
type Cmd struct {
	Name  string
	Args  []string
	Env   []string // appended to the parent environment
	Dir   string
	Stdin io.Reader
}

// Result is the outcome of a process that ran to completion.
type Result struct {
	Stdout, Stderr []byte
	ExitCode       int
}

// Runner starts external processes.
type Runner interface {
	// Run captures output. The error is non-nil only when the process could
	// not be started or was cancelled; a non-zero exit is reported in Result.
	Run(ctx context.Context, c Cmd) (Result, error)
	// Interactive attaches the process to the current terminal.
	Interactive(ctx context.Context, c Cmd) error
}

// OS runs real processes.
type OS struct{}

func (OS) command(ctx context.Context, c Cmd) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...)
	cmd.Dir = c.Dir
	if len(c.Env) > 0 {
		cmd.Env = append(os.Environ(), c.Env...)
	}
	cmd.Stdin = c.Stdin
	return cmd
}

// Run implements Runner.
func (o OS) Run(ctx context.Context, c Cmd) (Result, error) {
	cmd := o.command(ctx, c)
	// Own process group, so that cancelling kills grandchildren too (a shell
	// running `sleep` would otherwise keep the output pipes open). Not done for
	// Interactive, where the child must stay in the terminal's foreground group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = killGrace
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	var ee *exec.ExitError
	if errors.As(err, &ee) && ctx.Err() == nil {
		res.ExitCode = ee.ExitCode()
		return res, nil
	}
	if err != nil && ctx.Err() != nil {
		return res, ctx.Err()
	}
	return res, err
}

// Interactive implements Runner.
func (o OS) Interactive(ctx context.Context, c Cmd) error {
	cmd := o.command(ctx, c)
	if c.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	err := cmd.Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		// A shell exiting non-zero is the user's business, not a failure.
		return nil
	}
	return err
}
