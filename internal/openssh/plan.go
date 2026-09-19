package openssh

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/target"
)

type SSHOptions struct {
	User         string
	IdentityFile string
	Port         int
}

type SSHRequest struct {
	Target        target.ResolvedTarget
	Options       SSHOptions
	Executable    string
	Ssmm          string
	ExtraArgs     []string
	ProfileSource string
}

type SCPRequest struct {
	Target     target.ResolvedTarget
	Options    SSHOptions
	Executable string
	Ssmm       string
	Transfer   Transfer
	Recursive  bool
}

func PlanSSH(req SSHRequest) (execplan.ProcessSpec, error) {
	if err := req.Target.Validate(); err != nil {
		return execplan.ProcessSpec{}, err
	}
	ssh, err := executableOr(req.Executable, "ssh")
	if err != nil {
		return execplan.ProcessSpec{}, err
	}
	if req.Ssmm == "" {
		return execplan.ProcessSpec{}, fmt.Errorf("ssmm executable is required")
	}
	port := req.Options.Port
	if port == 0 {
		port = 22
	}
	proxy, err := ProxyCommand(req.Ssmm, req.Target.InstanceID, req.Target.Region, port, req.Target.Profile.Name, string(req.Target.Profile.Source), false)
	if err != nil {
		return execplan.ProcessSpec{}, err
	}
	args := []string{"-o", "ProxyCommand=" + proxy, "-o", "HostName=" + req.Target.InstanceID, "-o", "HostKeyAlias=ssmm-" + req.Target.Region + "-" + req.Target.InstanceID + "-" + strconv.Itoa(port), "-o", "ControlPath=none"}
	if req.Options.User != "" {
		args = append(args, "-o", "User="+req.Options.User)
	}
	if req.Options.IdentityFile != "" {
		encoded, err := EncodePercent(req.Options.IdentityFile)
		if err != nil {
			return execplan.ProcessSpec{}, err
		}
		if strings.Contains(encoded, "${") {
			return execplan.ProcessSpec{}, fmt.Errorf("identity file contains unsupported ${...}")
		}
		args = append(args, "-o", "IdentityFile="+encoded)
	}
	if port != 22 {
		args = append(args, "-p", strconv.Itoa(port))
	}
	// HostName is an option used for connection resolution; OpenSSH still
	// requires a destination operand to start a connection.
	args = append(args, req.Target.InstanceID)
	args = append(args, req.ExtraArgs...)
	return execplan.ProcessSpec{Executable: ssh, Args: args, Mode: execplan.Foreground, EnvironmentPolicy: policy(req.Target)}, nil
}

func PlanSCP(req SCPRequest) (execplan.ProcessSpec, error) {
	if err := req.Target.Validate(); err != nil {
		return execplan.ProcessSpec{}, err
	}
	scp, err := executableOr(req.Executable, "scp")
	if err != nil {
		return execplan.ProcessSpec{}, err
	}
	port := req.Options.Port
	if port == 0 {
		port = 22
	}
	proxy, err := ProxyCommand(req.Ssmm, req.Target.InstanceID, req.Target.Region, port, req.Target.Profile.Name, string(req.Target.Profile.Source), false)
	if err != nil {
		return execplan.ProcessSpec{}, err
	}
	args := []string{"-o", "ProxyCommand=" + proxy, "-o", "HostName=" + req.Target.InstanceID, "-o", "HostKeyAlias=ssmm-" + req.Target.Region + "-" + req.Target.InstanceID + "-" + strconv.Itoa(port), "-o", "ControlPath=none"}
	if req.Options.User != "" {
		args = append(args, "-o", "User="+req.Options.User)
	}
	if req.Options.IdentityFile != "" {
		encoded, err := EncodePercent(req.Options.IdentityFile)
		if err != nil {
			return execplan.ProcessSpec{}, err
		}
		if strings.Contains(encoded, "${") {
			return execplan.ProcessSpec{}, fmt.Errorf("identity file contains unsupported ${...}")
		}
		args = append(args, "-o", "IdentityFile="+encoded)
	}
	if port != 22 {
		args = append(args, "-P", strconv.Itoa(port))
	}
	if req.Recursive {
		args = append(args, "-r")
	}
	remote := req.Transfer.Remote
	if len(remote) == 0 {
		return execplan.ProcessSpec{}, fmt.Errorf("transfer has no remote operand")
	}
	user := req.Options.User
	if req.Transfer.User != "" {
		if user != "" && user != req.Transfer.User {
			return execplan.ProcessSpec{}, fmt.Errorf("USER@ target conflicts with --user")
		}
		user = req.Transfer.User
	}
	formatRemote := func(e Endpoint) string {
		prefix := ""
		if user != "" {
			prefix = user + "@"
		}
		return prefix + req.Target.InstanceID + ":" + e.Path
	}
	if req.Transfer.Direction == Send {
		for _, path := range req.Transfer.Local {
			if strings.HasPrefix(path, "-") {
				path = "./" + path
			}
			args = append(args, path)
		}
		args = append(args, formatRemote(remote[0]))
	} else {
		for _, e := range remote {
			args = append(args, formatRemote(e))
		}
		for _, path := range req.Transfer.Local {
			if strings.HasPrefix(path, "-") {
				path = "./" + path
			}
			args = append(args, path)
		}
	}
	return execplan.ProcessSpec{Executable: scp, Args: args, Mode: execplan.Foreground, EnvironmentPolicy: policy(req.Target)}, nil
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
