//go:build linux

package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/uncho/ssmm/internal/config"
	"golang.org/x/sys/unix"
)

// init の入力編集、キャンセル、端末設定の復元を実 PTY 上で検証する。
func TestInitPromptEditingAndCancellationWithTTY(t *testing.T) {
	if os.Getenv("SSMM_INIT_PTY_HELPER") == "1" {
		runInitPTYHelper(t)
		return
	}

	t.Setenv("TERM", "dumb")

	for _, mode := range []string{"edit", "cancel", "escape", "interrupt", "hangup"} {
		t.Run(mode, func(t *testing.T) {
			store, paths := sshConfigTestStore(t)
			pty := startInitPTYHelper(t, paths, mode)
			defer pty.close()

			if mode != "edit" {
				input := []byte{0x03}
				if mode == "escape" {
					input = []byte{0x1b}
				} else if mode == "interrupt" || mode == "hangup" {
					input = nil
				}
				if err := pty.answer("AWS profile (empty = environment/default)", input); err != nil {
					t.Fatal(err)
				}
				if mode == "interrupt" || mode == "hangup" {
					sig := syscall.SIGINT
					if mode == "hangup" {
						sig = syscall.SIGHUP
					}
					if err := pty.cmd.Process.Signal(sig); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				for _, step := range []struct {
					prompt string
					input  []byte
				}{
					{"AWS profile (empty = environment/default)", []byte("company-prodx\x08\r")},
					{"Regions (comma separated, empty = all)", []byte("us-east-1x\x7f\r")},
					{"SSH user", []byte("ubuntx\x08u\r")},
					{"SSH identity file", []byte("key .pxem\x1b[D\x1b[D\x1b[D\x1b[3~\r")},
					{"SSH port (empty = default)", []byte("\r")},
				} {
					if err := pty.answer(step.prompt, step.input); err != nil {
						t.Fatal(err)
					}
				}
			}

			if err := pty.wait(); err != nil {
				t.Fatal(err)
			}
			if mode != "edit" {
				if _, err := os.Stat(paths.ConfigFile); !os.IsNotExist(err) {
					t.Fatalf("canceled init wrote settings: %v", err)
				}
				return
			}

			snapshot, err := store.Read(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			profile := snapshot.Profile("prod")
			if profile.AWSProfile != "company-prod" || profile.Regions == nil || len(*profile.Regions) != 1 || (*profile.Regions)[0] != "us-east-1" {
				t.Fatalf("edited general settings = %#v", profile)
			}
			if profile.SSH.User != "ubuntu" || filepath.Base(profile.SSH.IdentityFile) != "key .pem" {
				t.Fatalf("edited SSH settings = %#v", profile.SSH)
			}
		})
	}
}

func runInitPTYHelper(t *testing.T) {
	t.Helper()
	paths := config.Paths{
		ConfigFile:     os.Getenv("SSMM_INIT_PTY_CONFIG"),
		SSHManagedFile: os.Getenv("SSMM_INIT_PTY_SSH_MANAGED"),
		SSHConfigFile:  os.Getenv("SSMM_INIT_PTY_SSH_CONFIG"),
		LockFile:       os.Getenv("SSMM_INIT_PTY_LOCK"),
	}
	before, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	command := New(Dependencies{Store: config.NewAppStore(paths, "/usr/local/bin/ssmm")})
	command.SetArgs([]string{"init", "prod", "--ssh"})
	err = command.Execute()
	if os.Getenv("SSMM_INIT_PTY_MODE") == "edit" && err != nil {
		t.Fatal(err)
	}
	if os.Getenv("SSMM_INIT_PTY_MODE") != "edit" {
		if err == nil || !strings.Contains(err.Error(), "canceled") {
			t.Fatalf("canceled init error = %v", err)
		}
	}
	after, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	if err != nil {
		t.Fatal(err)
	}
	if *before != *after {
		t.Fatalf("terminal settings were not restored: before=%+v after=%+v", *before, *after)
	}
}

type initPTYEvent struct {
	data []byte
	err  error
}

type initPTYProcess struct {
	master     *os.File
	cmd        *exec.Cmd
	events     chan initPTYEvent
	done       chan struct{}
	stop       chan struct{}
	readerDone chan struct{}
	waitErr    error
	output     bytes.Buffer
}

func startInitPTYHelper(t *testing.T, paths config.Paths, mode string) *initPTYProcess {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY|unix.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0); err != nil {
		_ = master.Close()
		t.Fatal(err)
	}
	ptyNumber, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	if err != nil {
		_ = master.Close()
		t.Fatal(err)
	}
	slave, err := os.OpenFile(filepath.Join("/dev/pts", fmt.Sprint(ptyNumber)), os.O_RDWR, 0)
	if err != nil {
		_ = master.Close()
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestInitPromptEditingAndCancellationWithTTY$", "-test.v")
	cmd.Env = append(os.Environ(),
		"SSMM_INIT_PTY_HELPER=1",
		"SSMM_INIT_PTY_MODE="+mode,
		"SSMM_INIT_PTY_CONFIG="+paths.ConfigFile,
		"SSMM_INIT_PTY_SSH_MANAGED="+paths.SSHManagedFile,
		"SSMM_INIT_PTY_SSH_CONFIG="+paths.SSHConfigFile,
		"SSMM_INIT_PTY_LOCK="+paths.LockFile,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	if err := cmd.Start(); err != nil {
		_ = slave.Close()
		_ = master.Close()
		t.Fatal(err)
	}
	_ = slave.Close()
	pty := &initPTYProcess{master: master, cmd: cmd, events: make(chan initPTYEvent), done: make(chan struct{}), stop: make(chan struct{}), readerDone: make(chan struct{})}
	go func() {
		defer close(pty.readerDone)
		defer close(pty.events)
		buffer := make([]byte, 4096)
		for {
			n, err := master.Read(buffer)
			if n > 0 {
				select {
				case pty.events <- initPTYEvent{data: append([]byte(nil), buffer[:n]...)}:
				case <-pty.stop:
					return
				}
			}
			if err != nil {
				select {
				case pty.events <- initPTYEvent{err: err}:
				case <-pty.stop:
				}
				return
			}
		}
	}()
	go func() {
		pty.waitErr = cmd.Wait()
		close(pty.done)
	}()
	return pty
}

func (p *initPTYProcess) answer(prompt string, input []byte) error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for !bytes.Contains(p.output.Bytes(), []byte(prompt)) {
		select {
		case event, ok := <-p.events:
			if !ok || event.err != nil {
				return fmt.Errorf("PTY closed before prompt %q: %w; output: %s", prompt, event.err, p.output.String())
			}
			p.output.Write(event.data)
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for prompt %q; output: %s", prompt, p.output.String())
		}
	}
	_, err := p.master.Write(input)
	return err
}

func (p *initPTYProcess) wait() error {
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	events := p.events
	for {
		select {
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			p.output.Write(event.data)
		case <-p.done:
			if p.waitErr != nil {
				return fmt.Errorf("init PTY helper failed: %w; output: %s", p.waitErr, p.output.String())
			}
			return nil
		case <-deadline.C:
			return fmt.Errorf("init PTY helper did not exit; output: %s", p.output.String())
		}
	}
}

func (p *initPTYProcess) close() {
	select {
	case <-p.done:
	default:
		_ = p.cmd.Process.Kill()
		<-p.done
	}
	close(p.stop)
	_ = p.master.Close()
	<-p.readerDone
}
