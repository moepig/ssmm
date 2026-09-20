package session

import (
	"fmt"
	"os/exec"
	"strconv"

	"github.com/uncho/ssmm/internal/execplan"
	"github.com/uncho/ssmm/internal/target"
)

type Request struct {
	Target target.ResolvedTarget
	AWS    string
	Proxy  bool
	Port   int
}

func Plan(req Request) (execplan.ProcessSpec, error) {
	if err := req.Target.Validate(); err != nil {
		return execplan.ProcessSpec{}, err
	}
	awsPath := req.AWS
	if awsPath == "" {
		var err error
		awsPath, err = exec.LookPath("aws")
		if err != nil {
			return execplan.ProcessSpec{}, fmt.Errorf("aws CLI not found: %w", err)
		}
	}
	if req.Proxy && (req.Port < 1 || req.Port > 65535) {
		return execplan.ProcessSpec{}, fmt.Errorf("proxy port is out of range")
	}
	args := []string{"ssm", "start-session", "--target", req.Target.InstanceID, "--region", req.Target.Region}
	if req.Proxy {
		args = append(args, "--document-name", "AWS-StartSSHSession", "--parameters", `{"portNumber":["`+strconv.Itoa(req.Port)+`"]}`)
	}
	if req.Target.Profile.Source == target.ProfileConfig || req.Target.Profile.Source == target.ProfileFlag || req.Target.Profile.Source == target.ProfileHost {
		args = append(args, "--profile", req.Target.Profile.Name)
	}
	return execplan.ProcessSpec{Executable: awsPath, Args: args, Mode: func() execplan.Mode {
		if req.Proxy {
			return execplan.ProxyStream
		}
		return execplan.Foreground
	}(), EnvironmentPolicy: execplan.EnvironmentPolicy{UnsetPager: true, DisableAutoPrompt: true, Profile: req.Target.Profile.Name, ProfileSource: string(req.Target.Profile.Source)}}, nil
}

func CheckDependencies() error {
	for _, name := range []string{"aws", "session-manager-plugin"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("%s not found: %w", name, err)
		}
	}
	return nil
}
