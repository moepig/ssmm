package openssh

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/uncho/ssmm/internal/app"
	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/target"
)

type Planner struct{}

func (Planner) PlanSSH(req app.SSHRequest) (execplan.ProcessSpec, error) {
	if err := req.Options.Validate(); err != nil {
		return execplan.ProcessSpec{}, err
	}
	if err := req.Target.Validate(); err != nil {
		return execplan.ProcessSpec{}, err
	}
	ssh, err := executableOr(req.Executable, "ssh")
	if err != nil {
		return execplan.ProcessSpec{}, err
	}
	args, port, err := connectionOptions(req.Target, req.Options, req.SsmmExecutable)
	if err != nil {
		return execplan.ProcessSpec{}, err
	}
	if port != 22 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	args = append(args, req.Target.InstanceID)
	args = append(args, req.ExtraArgs...)
	return execplan.ProcessSpec{Executable: ssh, Args: args, Mode: execplan.Foreground, EnvironmentPolicy: policy(req.Target)}, nil
}

func (Planner) PlanSCP(req app.SCPRequest) (execplan.ProcessSpec, error) {
	if err := req.Options.Validate(); err != nil {
		return execplan.ProcessSpec{}, err
	}
	if err := req.Target.Validate(); err != nil {
		return execplan.ProcessSpec{}, err
	}
	scp, err := executableOr(req.Executable, "scp")
	if err != nil {
		return execplan.ProcessSpec{}, err
	}
	args, port, err := connectionOptions(req.Target, req.Options, req.SsmmExecutable)
	if err != nil {
		return execplan.ProcessSpec{}, err
	}
	if port != 22 {
		args = append(args, "-P", strconv.Itoa(port))
	}
	if req.Recursive {
		args = append(args, "-r")
	}
	if err := appendTransferArgs(&args, req); err != nil {
		return execplan.ProcessSpec{}, err
	}
	return execplan.ProcessSpec{Executable: scp, Args: args, Mode: execplan.Foreground, EnvironmentPolicy: policy(req.Target)}, nil
}

func connectionOptions(resolved target.ResolvedTarget, options app.SSHOptions, ssmm string) ([]string, int, error) {
	if ssmm == "" {
		return nil, 0, fmt.Errorf("ssmm executable is required")
	}
	port := options.Port
	if port == 0 {
		port = 22
	}
	proxy, err := ProxyCommand(ssmm, resolved.InstanceID, resolved.Region, port, resolved.Profile.Name, string(resolved.Profile.Source))
	if err != nil {
		return nil, 0, err
	}
	args := []string{"-o", "ProxyCommand=" + proxy, "-o", "HostName=" + resolved.InstanceID, "-o", "HostKeyAlias=ssmm-" + resolved.Region + "-" + resolved.InstanceID + "-" + strconv.Itoa(port), "-o", "ControlPath=none"}
	if options.User != "" {
		value, err := QuoteConfigValue(options.User)
		if err != nil {
			return nil, 0, fmt.Errorf("quote SSH user: %w", err)
		}
		args = append(args, "-o", "User="+value)
	}
	if options.IdentityFile != "" {
		value, err := QuoteIdentityFile(options.IdentityFile)
		if err != nil {
			return nil, 0, err
		}
		args = append(args, "-o", "IdentityFile="+configValueForArgument(value, options.IdentityFile))
	}
	return args, port, nil
}

func configValueForArgument(quoted, original string) string {
	if strings.IndexFunc(original, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\\' || r == '"' || r == '\'' || r == '#'
	}) < 0 {
		return strings.Trim(quoted, `"`)
	}
	return quoted
}

func appendTransferArgs(args *[]string, req app.SCPRequest) error {
	transfer := req.Transfer
	if transfer.Direction != app.SCPSend && transfer.Direction != app.SCPReceive {
		return fmt.Errorf("invalid SCP transfer direction %q", transfer.Direction)
	}
	if len(transfer.Remote) == 0 {
		return fmt.Errorf("transfer has no remote operand")
	}
	user := req.Options.User
	if transfer.User != "" {
		if user != "" && user != transfer.User {
			return fmt.Errorf("USER@ target conflicts with --user")
		}
		user = transfer.User
	}
	if err := (app.SSHOptions{User: user}).Validate(); err != nil {
		return err
	}
	formatRemote := func(remote app.SCPRemote) string {
		prefix := ""
		if user != "" {
			prefix = user + "@"
		}
		return prefix + req.Target.InstanceID + ":" + remote.Path
	}
	localPath := func(path string) string {
		if strings.HasPrefix(path, "-") {
			return "./" + path
		}
		return path
	}
	if transfer.Direction == app.SCPSend {
		for _, path := range transfer.Local {
			*args = append(*args, localPath(path))
		}
		*args = append(*args, formatRemote(transfer.Remote[0]))
		return nil
	}
	for _, remote := range transfer.Remote {
		*args = append(*args, formatRemote(remote))
	}
	for _, path := range transfer.Local {
		*args = append(*args, localPath(path))
	}
	return nil
}

func executableOr(value, name string) (string, error) {
	if value != "" {
		return value, nil
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s not found: %w", name, err)
	}
	return path, nil
}

func policy(t target.ResolvedTarget) execplan.EnvironmentPolicy {
	return execplan.EnvironmentPolicy{UnsetPager: true, DisableAutoPrompt: true, Profile: t.Profile.Name, ProfileSource: string(t.Profile.Source)}
}
