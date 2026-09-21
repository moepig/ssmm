package proxy

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/uncho/ssmm/internal/target"
)

type Host struct {
	Target  string
	Profile string
}

var regionRE = regexp.MustCompile(`^[a-z]{2}(?:-[a-z]+)+-\d+$`)

func ParseHost(host string) (Host, error) {
	if !strings.HasSuffix(host, ".ssmm") {
		return Host{}, fmt.Errorf("host %q does not end in .ssmm", host)
	}
	body := strings.TrimSuffix(host, ".ssmm")
	idx := strings.LastIndexByte(body, '.')
	if idx <= 0 || idx == len(body)-1 {
		return Host{}, fmt.Errorf("invalid ssmm host %q", host)
	}
	targetName, profile := body[:idx], body[idx+1:]
	if err := target.ValidateSSHLabel(profile); err != nil {
		return Host{}, err
	}
	parts := strings.Split(targetName, ".")
	for _, part := range parts {
		if err := target.ValidateSSHLabel(part); err != nil {
			return Host{}, fmt.Errorf("invalid target label %q", part)
		}
	}
	return Host{Target: targetName, Profile: profile}, nil
}

type InternalRequest struct {
	InstanceID string
	Region     string
	Port       int
	Profile    string
	Source     string
}

func (r InternalRequest) Validate(environProfile string) error {
	if !target.IsInstanceID(r.InstanceID) {
		return fmt.Errorf("invalid internal proxy instance ID")
	}
	if !regionRE.MatchString(r.Region) || strings.HasPrefix(r.Region, "cn-") || strings.Contains(r.Region, "-gov-") {
		return fmt.Errorf("invalid internal proxy region")
	}
	if r.Port < 1 || r.Port > 65535 {
		return fmt.Errorf("invalid internal proxy port")
	}
	if r.Profile == "" {
		return fmt.Errorf("internal proxy profile is empty")
	}
	switch r.Source {
	case "config", "flag", "host":
	case "env":
		if environProfile == "" || environProfile != r.Profile {
			return fmt.Errorf("internal proxy environment profile does not match")
		}
	case "default":
		if r.Profile != "default" || environProfile != "" {
			return fmt.Errorf("invalid internal default profile")
		}
	default:
		return fmt.Errorf("invalid internal proxy profile source %q", r.Source)
	}
	return nil
}
