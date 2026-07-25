package process_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"mlc-cli/internal/process"
)

// TestHelperProcess is used as a mock subprocess for testing.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		os.Exit(0)
	}

	cmd := args[0]
	switch cmd {
	case "echo-args":
		for i, arg := range args[1:] {
			fmt.Printf("ARG[%d]=%s\n", i, arg)
		}
		os.Exit(0)
	case "stdout-stderr":
		fmt.Fprint(os.Stdout, "sample stdout output")
		fmt.Fprint(os.Stderr, "sample stderr output")
		os.Exit(0)
	case "nonzero-exit":
		fmt.Fprint(os.Stderr, "failing process error")
		os.Exit(42)
	case "check-env":
		key := args[1]
		fmt.Fprint(os.Stdout, os.Getenv(key))
		os.Exit(0)
	case "sleep":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		os.Exit(2)
	}
}

func helperArgs(command string, extra ...string) []string {
	args := []string{"-test.run=TestHelperProcess", "--", command}
	return append(args, extra...)
}

func helperEnv() []string {
	return []string{"GO_WANT_HELPER_PROCESS=1"}
}

func TestArgumentPreservationWithSpaces(t *testing.T) {
	var stdout bytes.Buffer
	err := process.Run(
		os.Args[0],
		helperArgs("echo-args", "first arg", "second arg with spaces"),
		process.WithEnv(helperEnv()),
		process.WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "ARG[0]=first arg") {
		t.Errorf("expected stdout to contain 'ARG[0]=first arg', got:\n%s", out)
	}
	if !strings.Contains(out, "ARG[1]=second arg with spaces") {
		t.Errorf("expected stdout to contain 'ARG[1]=second arg with spaces', got:\n%s", out)
	}
}

func TestStdoutPreserved(t *testing.T) {
	var stdout bytes.Buffer
	err := process.Run(
		os.Args[0],
		helperArgs("stdout-stderr"),
		process.WithEnv(helperEnv()),
		process.WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "sample stdout output" {
		t.Errorf("expected 'sample stdout output', got %q", stdout.String())
	}
}

func TestStderrPreserved(t *testing.T) {
	var stderr bytes.Buffer
	err := process.Run(
		os.Args[0],
		helperArgs("stdout-stderr"),
		process.WithEnv(helperEnv()),
		process.WithStderr(&stderr),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stderr.String() != "sample stderr output" {
		t.Errorf("expected 'sample stderr output', got %q", stderr.String())
	}
}

func TestNonZeroExitCodeRecoverable(t *testing.T) {
	var stderr bytes.Buffer
	err := process.Run(
		os.Args[0],
		helperArgs("nonzero-exit"),
		process.WithEnv(helperEnv()),
		process.WithStderr(&stderr),
	)
	if err == nil {
		t.Fatal("expected error for nonzero exit code, got nil")
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 42 {
		t.Errorf("expected exit code 42, got %d", exitErr.ExitCode())
	}
	if stderr.String() != "failing process error" {
		t.Errorf("expected stderr 'failing process error', got %q", stderr.String())
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := process.Run(
		os.Args[0],
		helperArgs("sleep"),
		process.WithEnv(helperEnv()),
		process.WithContext(ctx),
	)
	if err == nil {
		t.Fatal("expected error due to context cancellation, got nil")
	}
}

func TestEnvironmentVariablesReachSubprocess(t *testing.T) {
	var stdout bytes.Buffer
	err := process.Run(
		os.Args[0],
		helperArgs("check-env", "TEST_CUSTOM_VAR"),
		process.WithEnv(append(helperEnv(), "TEST_CUSTOM_VAR=hello_world_123")),
		process.WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stdout.String() != "hello_world_123" {
		t.Errorf("expected 'hello_world_123', got %q", stdout.String())
	}
}

func TestOutputCapturesStdoutWithoutMixingStderr(t *testing.T) {
	var stderr bytes.Buffer

	stdout, err := process.Output(
		os.Args[0],
		helperArgs("stdout-stderr"),
		process.WithEnv(helperEnv()),
		process.WithStderr(&stderr),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(stdout) != "sample stdout output" {
		t.Errorf("expected stdout %q, got %q", "sample stdout output", string(stdout))
	}

	if stderr.String() != "sample stderr output" {
		t.Errorf("expected stderr %q, got %q", "sample stderr output", stderr.String())
	}
}

func TestExistingEnvironmentIsPreserved(t *testing.T) {
	const key = "PATH"

	expected := os.Getenv(key)
	if expected == "" {
		t.Skip("PATH is empty in the test environment")
	}

	var stdout bytes.Buffer

	err := process.Run(
		os.Args[0],
		helperArgs("check-env", key),
		process.WithEnv(helperEnv()),
		process.WithStdout(&stdout),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if stdout.String() != expected {
		t.Errorf("expected inherited %s value %q, got %q", key, expected, stdout.String())
	}
}
