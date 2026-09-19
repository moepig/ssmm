//go:build linux

package process

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
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
			Args:       []string{"-c", "read value; test \"$value\" = foreground"},
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
	var output bytes.Buffer
	process.Stdout = &output
	process.Stderr = &output
	input, err := process.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := input.Write([]byte("foreground\n")); err != nil {
		t.Fatal(err)
	}
	_ = input.Close()
	done := make(chan error, 1)
	go func() { done <- process.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("PTY helper failed: %v\n%s", err, output.String())
		}
	case <-time.After(3 * time.Second):
		_ = process.Process.Kill()
		t.Fatalf("PTY helper hung; child likely did not receive the foreground terminal\n%s", output.String())
	}
	if strings.Contains(output.String(), "PTY child did not receive input") {
		t.Fatalf("helper reported foreground failure: %s", output.String())
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
