//go:build linux

package process

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uncho/ssmm/internal/execplan"
	"golang.org/x/sys/unix"
)

func TestRunnerGivesForegroundPTYToChild(t *testing.T) {
	if os.Getenv("SSMM_PTY_HELPER") == "1" {
		original, err := unix.IoctlGetInt(int(os.Stdin.Fd()), unix.TIOCGPGRP)
		if err != nil {
			t.Fatal(err)
		}
		result, err := (NewRunner()).Run(t.Context(), execplan.ProcessSpec{
			Executable: "/bin/sh",
			Args:       []string{"-c", "printf 'PTY_READY\\n'; read value; test \"$value\" = foreground"},
			Mode:       execplan.Foreground,
		}, execplan.Streams{})
		if err != nil {
			t.Fatal(err)
		}
		if result.ExitCode != 0 {
			t.Fatalf("PTY child did not receive input: %#v", result)
		}
		restored, err := unix.IoctlGetInt(int(os.Stdin.Fd()), unix.TIOCGPGRP)
		if err != nil {
			t.Fatal(err)
		}
		if restored != original {
			t.Fatalf("terminal foreground group was not restored: before=%d after=%d", original, restored)
		}
		return
	}
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script is unavailable")
	}

	command := shellQuote(os.Args[0]) + " -test.run=^TestRunnerGivesForegroundPTYToChild$ -test.v"
	process := exec.Command("script", "-qfec", command, "/dev/null")
	process.Env = append(os.Environ(), "SSMM_PTY_HELPER=1")
	output := &ptyTestOutput{ready: make(chan struct{})}
	process.Stdout = output
	process.Stderr = output
	input, err := process.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- process.Wait() }()
	select {
	case <-output.ready:
	case err := <-done:
		t.Fatalf("PTY helper exited before child was ready: %v\n%s", err, output.String())
	case <-time.After(10 * time.Second):
		_ = process.Process.Kill()
		t.Fatalf("PTY child did not become ready\n%s", output.String())
	}
	if _, err := input.Write([]byte("foreground\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		_ = input.Close()
		if err != nil {
			t.Fatalf("PTY helper failed: %v\n%s", err, output.String())
		}
	case <-time.After(10 * time.Second):
		_ = process.Process.Kill()
		_ = input.Close()
		t.Fatalf("PTY helper hung; child likely did not receive the foreground terminal\n%s", output.String())
	}
	if strings.Contains(output.String(), "PTY child did not receive input") {
		t.Fatalf("helper reported foreground failure: %s", output.String())
	}
}

type ptyTestOutput struct {
	sync.Mutex
	buffer bytes.Buffer
	ready  chan struct{}
	once   sync.Once
}

func (output *ptyTestOutput) Write(p []byte) (int, error) {
	output.Lock()
	defer output.Unlock()
	n, err := output.buffer.Write(p)
	if bytes.Contains(output.buffer.Bytes(), []byte("PTY_READY")) {
		output.once.Do(func() { close(output.ready) })
	}
	return n, err
}

func (output *ptyTestOutput) String() string {
	output.Lock()
	defer output.Unlock()
	return output.buffer.String()
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
