//go:build unix

package process

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/uncho/ssmm/internal/execplan"
)

func TestRunnerPreservesExternalExitCode(t *testing.T) {
	runner := NewRunner()
	result, err := runner.Run(context.Background(), execplan.ProcessSpec{Executable: "/bin/sh", Args: []string{"-c", "exit 7"}, Mode: execplan.ProxyStream}, execplan.Streams{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 7 || !result.Started {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunnerStopsProcessGroupOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := NewRunner()
	runner.GracePeriod = 100 * time.Millisecond
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		_, _ = runner.Run(ctx, execplan.ProcessSpec{Executable: "/bin/sh", Args: []string{"-c", "sleep 10"}, Mode: execplan.ProxyStream}, execplan.Streams{})
		close(done)
	}()
	<-started
	time.Sleep(20 * time.Millisecond)
	cancel()
	// The runner's bounded grace period is the observable contract. The test
	// only verifies that cancellation returns instead of waiting for sleep.
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not return after cancellation")
	}
}

func TestRunnerBoundsShutdownAfterExternalSignal(t *testing.T) {
	runner := NewRunner()
	runner.GracePeriod = 80 * time.Millisecond
	started := time.Now()
	result, err := runner.Run(context.Background(), execplan.ProcessSpec{
		Executable: "/bin/sh",
		Args:       []string{"-c", "trap '' TERM HUP; kill -TERM $PPID; sleep 10"},
		Mode:       execplan.ProxyStream,
	}, execplan.Streams{})
	if err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("runner waited too long after external signal: %s", elapsed)
	}
	if result.RequestedStop != "terminated" {
		t.Fatalf("unexpected stop request: %#v", result)
	}
}
