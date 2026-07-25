package process

import (
	"context"
	"io"
	"os/exec"
)

// Option configures a subprocess execution.
type Option func(*options)

type options struct {
	ctx    context.Context
	dir    string
	env    []string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// WithContext sets the cancellation context.
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		o.ctx = ctx
	}
}

// WithDir sets the subprocess working directory.
func WithDir(dir string) Option {
	return func(o *options) {
		o.dir = dir
	}
}

// WithEnv appends environment variables in KEY=VALUE format.
//
// The existing process environment is preserved.
func WithEnv(env []string) Option {
	return func(o *options) {
		o.env = append(o.env, env...)
	}
}

// WithStdin sets the subprocess standard input.
func WithStdin(r io.Reader) Option {
	return func(o *options) {
		o.stdin = r
	}
}

// WithStdout sets the subprocess standard output.
func WithStdout(w io.Writer) Option {
	return func(o *options) {
		o.stdout = w
	}
}

// WithStderr sets the subprocess standard error.
func WithStderr(w io.Writer) Option {
	return func(o *options) {
		o.stderr = w
	}
}

// Run executes a program directly with an argument slice.
//
// It does not invoke a shell or rewrite subprocess errors.
func Run(name string, args []string, opts ...Option) error {
	return buildCommand(name, args, opts...).Run()
}

// Output executes a program and returns its standard output.
//
// Standard error remains separate and can be configured with WithStderr.
func Output(name string, args []string, opts ...Option) ([]byte, error) {
	return buildCommand(name, args, opts...).Output()
}

func buildCommand(name string, args []string, opts ...Option) *exec.Cmd {
	var cfg options
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	var cmd *exec.Cmd
	if cfg.ctx != nil {
		cmd = exec.CommandContext(cfg.ctx, name, args...)
	} else {
		cmd = exec.Command(name, args...)
	}

	cmd.Dir = cfg.dir

	if len(cfg.env) > 0 {
		cmd.Env = append(cmd.Environ(), cfg.env...)
	}

	cmd.Stdin = cfg.stdin
	cmd.Stdout = cfg.stdout
	cmd.Stderr = cfg.stderr

	return cmd
}
