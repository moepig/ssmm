//go:build linux

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/uncho/ssmm/internal/app"
	"github.com/uncho/ssmm/internal/target"
	"github.com/uncho/ssmm/internal/terminal"
	"golang.org/x/sys/unix"
)

// PTY 上で TARGET を渡したとき、端末選択 UI を開かず一意な対象へ接続する。
func TestExplicitTargetUsesNonInteractivePathWithTTY(t *testing.T) {
	if os.Getenv("SSMM_EXPLICIT_TARGET_PTY_HELPER") == "1" {
		if !terminal.Available() {
			t.Fatal("helper does not have a controlling terminal")
		}
		setupSelectionExecutables(t)
		backend := &selectionBackend{instances: []target.Instance{selectionInstance("web")}}
		planner := &selectionPlanner{}
		command := New(Dependencies{
			Service:  app.Service{Profiles: backend, AWS: backend, Session: planner},
			Profiles: backend, Runner: planner,
		})
		command.SetArgs([]string{"web"})
		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if !planner.ran {
			t.Fatal("connection did not run")
		}
		return
	}
	t.Setenv("TERM", "dumb")

	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	ptyNumber, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	slave, err := os.OpenFile(filepath.Join("/dev/pts", fmt.Sprint(ptyNumber)), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer slave.Close()

	process := exec.Command(os.Args[0], "-test.run=^TestExplicitTargetUsesNonInteractivePathWithTTY$", "-test.v")
	process.Env = append(os.Environ(), "SSMM_EXPLICIT_TARGET_PTY_HELPER=1")
	process.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	process.Stdin, process.Stdout, process.Stderr = slave, slave, slave
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	_ = slave.Close()
	done := make(chan error, 1)
	go func() { done <- process.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("PTY helper failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		_ = process.Process.Kill()
		<-done
		t.Fatal("PTY helper hung, likely waiting for target selection")
	}
}
