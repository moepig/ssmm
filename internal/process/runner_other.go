//go:build !unix

package process

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/uncho/ssmm/internal/execplan"
)

type Runner struct{}

func NewRunner() Runner { return Runner{} }
func (Runner) Run(ctx context.Context, spec execplan.ProcessSpec, streams execplan.Streams) (execplan.ProcessResult, error) {
	if err := spec.Validate(); err != nil {
		return execplan.ProcessResult{}, err
	}
	cmd := exec.CommandContext(ctx, spec.Executable, spec.Args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = streams.Stdin, streams.Stdout, streams.Stderr
	if cmd.Stdin == nil {
		cmd.Stdin = os.Stdin
	}
	if cmd.Stdout == nil {
		cmd.Stdout = os.Stdout
	}
	if cmd.Stderr == nil {
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return execplan.ProcessResult{Started: true, ExitCode: ee.ExitCode()}, nil
		}
		return execplan.ProcessResult{}, fmt.Errorf("run process: %w", err)
	}
	return execplan.ProcessResult{Started: true, ExitCode: 0}, nil
}
