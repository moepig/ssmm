package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/uncho/ssmm/internal/app"
	"github.com/uncho/ssmm/internal/awsconfig"
	"github.com/uncho/ssmm/internal/cli"
	"github.com/uncho/ssmm/internal/config"
	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/openssh"
	"github.com/uncho/ssmm/internal/process"
	"github.com/uncho/ssmm/internal/session"
	"github.com/uncho/ssmm/internal/target"
)

type awsFactory struct{ factory awsconfig.Factory }

func (f awsFactory) Open(ctx context.Context, selection target.ProfileSelection) (app.AWSRuntime, error) {
	return f.factory.Open(ctx, selection)
}

type sessionPlanner struct{ aws string }

func (p sessionPlanner) PlanSession(request app.SessionRequest) (execplan.ProcessSpec, error) {
	return session.Plan(session.Request{Target: request.Target, AWS: p.aws, Proxy: request.Proxy, Port: request.Port})
}

type sshPlanner struct{}

func (sshPlanner) PlanSSH(request app.SSHRequest) (execplan.ProcessSpec, error) {
	return openssh.PlanSSH(openssh.SSHRequest{Target: request.Target, Options: openssh.SSHOptions{User: request.Options.User, IdentityFile: request.Options.IdentityFile, Port: request.Options.Port}, Executable: request.Executable, Ssmm: request.SsmmExecutable, ExtraArgs: request.ExtraArgs})
}
func (sshPlanner) PlanSCP(request app.SCPRequest) (execplan.ProcessSpec, error) {
	transfer := openssh.Transfer{User: request.Transfer.User, Target: request.Transfer.Target, Local: append([]string(nil), request.Transfer.Local...)}
	if request.Transfer.Direction == app.SCPSend {
		transfer.Direction = openssh.Send
	} else {
		transfer.Direction = openssh.Receive
	}
	for _, remote := range request.Transfer.Remote {
		transfer.Remote = append(transfer.Remote, openssh.Endpoint{User: remote.User, Target: remote.Target, Path: remote.Path})
	}
	return openssh.PlanSCP(openssh.SCPRequest{Target: request.Target, Options: openssh.SSHOptions{User: request.Options.User, IdentityFile: request.Options.IdentityFile, Port: request.Options.Port}, Executable: request.Executable, Ssmm: request.SsmmExecutable, Transfer: transfer, Recursive: request.Recursive})
}

func main() {
	paths, err := config.DefaultPaths()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ssmmPath, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	store := config.NewAppStore(paths, ssmmPath)
	runner := process.NewRunner()
	service := app.Service{Settings: store, Profiles: awsconfig.ProfileChecker{}, AWS: awsFactory{factory: awsconfig.Factory{}}, Session: sessionPlanner{aws: executablePath("aws")}, SSH: sshPlanner{}, Runner: runner}
	command := cli.New(cli.Dependencies{Service: service, Store: store, Profiles: awsconfig.ProfileChecker{}, Runner: runner, SSMMExecutable: ssmmPath, Input: os.Stdin, Output: os.Stdout, Error: os.Stderr})
	if err := command.Execute(); err != nil {
		code := 1
		var exitErr *cli.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.Code
		}
		var appErr *app.AppError
		if errors.As(err, &appErr) && appErr.Kind == app.ErrInvalid {
			code = 2
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(code)
	}
}

func executablePath(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}
