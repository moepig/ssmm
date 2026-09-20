//go:build unix

package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/target"
	"golang.org/x/sys/unix"
)

type Runner struct{ GracePeriod time.Duration }

func NewRunner() Runner { return Runner{GracePeriod: 2 * time.Second} }

func (r Runner) Run(ctx context.Context, spec execplan.ProcessSpec, streams execplan.Streams) (execplan.ProcessResult, error) {
	if err := spec.Validate(); err != nil {
		return execplan.ProcessResult{}, err
	}
	if r.GracePeriod <= 0 {
		r.GracePeriod = 2 * time.Second
	}
	cmd := exec.Command(spec.Executable, spec.Args...)
	in, out, errOut := chooseStreams(streams)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, errOut
	cmd.Env = environment(spec.EnvironmentPolicy)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return execplan.ProcessResult{}, fmt.Errorf("start %s: %w", spec.Executable, err)
	}
	terminal := claimTerminal(in, spec.Mode, cmd.Process.Pid)
	defer terminal.restore()
	result := execplan.ProcessResult{Started: true, ExitCode: -1}
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(signals)
	ctxDone := ctx.Done()
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	stopSent := ""
	var stopTimer *time.Timer
	var stopTimerC <-chan time.Time
	forward := func(sig syscall.Signal) {
		if stopSent == "" {
			stopSent = sig.String()
		}
		_ = syscall.Kill(-cmd.Process.Pid, sig)
	}
	requestStop := func(sig syscall.Signal) {
		forward(sig)
		if stopTimer != nil {
			return
		}
		stopTimer = time.NewTimer(r.GracePeriod)
		stopTimerC = stopTimer.C
	}
	for {
		select {
		case err := <-wait:
			if stopTimer != nil {
				if !stopTimer.Stop() {
					select {
					case <-stopTimer.C:
					default:
					}
				}
			}
			cleanupProcessGroup(cmd.Process.Pid, r.GracePeriod)
			fillResult(&result, cmd.ProcessState, err)
			result.RequestedStop = stopSent
			return result, nil
		case sig := <-signals:
			requestStop(sig.(syscall.Signal))
		case <-ctxDone:
			requestStop(syscall.SIGTERM)
			ctxDone = nil
		case <-stopTimerC:
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			err := <-wait
			cleanupProcessGroup(cmd.Process.Pid, r.GracePeriod)
			fillResult(&result, cmd.ProcessState, err)
			result.RequestedStop = stopSent
			return result, nil
		}
	}
}

type terminalClaim struct {
	file          *os.File
	pgid          int
	claimed       bool
	restoreSignal bool
}

func claimTerminal(file *os.File, mode execplan.Mode, childPID int) terminalClaim {
	claim := terminalClaim{file: file}
	if mode != execplan.Foreground || file == nil {
		return claim
	}
	original, err := unix.IoctlGetInt(int(file.Fd()), unix.TIOCGPGRP)
	if err != nil {
		return claim
	}
	// The runner is temporarily in the background while the child owns the
	// terminal. Ignore SIGTTOU so restoring the terminal cannot stop the
	// runner itself when the child exits.
	claim.restoreSignal = !signal.Ignored(syscall.SIGTTOU)
	signal.Ignore(syscall.SIGTTOU)
	if err := unix.IoctlSetPointerInt(int(file.Fd()), unix.TIOCSPGRP, childPID); err != nil {
		if claim.restoreSignal {
			signal.Reset(syscall.SIGTTOU)
		}
		claim.restoreSignal = false
		return claim
	}
	claim.pgid = original
	claim.claimed = true
	return claim
}

func (claim terminalClaim) restore() {
	if !claim.claimed || claim.file == nil {
		if claim.restoreSignal {
			signal.Reset(syscall.SIGTTOU)
		}
		return
	}
	_ = unix.IoctlSetPointerInt(int(claim.file.Fd()), unix.TIOCSPGRP, claim.pgid)
	if claim.restoreSignal {
		signal.Reset(syscall.SIGTTOU)
	}
}

func cleanupProcessGroup(pgid int, grace time.Duration) {
	if syscall.Kill(-pgid, 0) != nil {
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if syscall.Kill(-pgid, 0) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}

func chooseStreams(s execplan.Streams) (*os.File, *os.File, *os.File) {
	in, out, err := os.Stdin, os.Stdout, os.Stderr
	if s.Stdin != nil {
		in = s.Stdin
	}
	if s.Stdout != nil {
		out = s.Stdout
	}
	if s.Stderr != nil {
		err = s.Stderr
	}
	return in, out, err
}

func environment(policy execplan.EnvironmentPolicy) []string {
	base := os.Environ()
	out := make([]string, 0, len(base)+3)
	for _, item := range base {
		if strings.HasPrefix(item, "AWS_DEFAULT_PROFILE=") {
			continue
		}
		if (policy.ProfileSource == string(target.ProfileConfig) || policy.ProfileSource == string(target.ProfileFlag) || policy.ProfileSource == string(target.ProfileHost) || policy.ProfileSource == string(target.ProfileDefault)) && strings.HasPrefix(item, "AWS_PROFILE=") {
			continue
		}
		if policy.UnsetPager && strings.HasPrefix(item, "AWS_PAGER=") {
			continue
		}
		if policy.DisableAutoPrompt && strings.HasPrefix(item, "AWS_CLI_AUTO_PROMPT=") {
			continue
		}
		out = append(out, item)
	}
	if policy.UnsetPager {
		out = append(out, "AWS_PAGER=")
	}
	if policy.DisableAutoPrompt {
		out = append(out, "AWS_CLI_AUTO_PROMPT=off")
	}
	return out
}

func fillResult(result *execplan.ProcessResult, state *os.ProcessState, err error) {
	if state == nil {
		result.Diagnostic = err
		return
	}
	if status, ok := state.Sys().(syscall.WaitStatus); ok {
		if status.Exited() {
			result.ExitCode = status.ExitStatus()
		}
		if status.Signaled() {
			result.Signal = status.Signal().String()
			result.ExitCode = 128 + int(status.Signal())
		}
	}
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			result.Diagnostic = err
		}
	}
}
