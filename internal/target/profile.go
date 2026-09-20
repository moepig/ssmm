package target

import (
	"fmt"
	"regexp"
	"strings"
)

// ProfileSource records how a profile was selected. The source is preserved
// because AWS SDK and AWS CLI resolve explicit and environment profiles
// differently.
type ProfileSource string

const (
	ProfileConfig  ProfileSource = "config"
	ProfileFlag    ProfileSource = "flag"
	ProfileHost    ProfileSource = "host"
	ProfileEnv     ProfileSource = "env"
	ProfileDefault ProfileSource = "default"
)

type ProfileSelection struct {
	Name   string
	Source ProfileSource
}

func (p ProfileSelection) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile name is empty")
	}
	if err := ValidateProfileName(p.Name); err != nil {
		return err
	}
	switch p.Source {
	case ProfileConfig, ProfileFlag, ProfileHost, ProfileEnv, ProfileDefault:
	default:
		return fmt.Errorf("invalid profile source %q", p.Source)
	}
	if p.Source == ProfileDefault && p.Name != "default" {
		return fmt.Errorf("default profile source requires profile name default")
	}
	return nil
}

var profileNameRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@+-]*$`)

func ValidateProfileName(name string) error {
	if name == "" || !profileNameRE.MatchString(name) {
		return fmt.Errorf("invalid profile name %q", name)
	}
	if strings.ContainsAny(name, "\x00\r\n") {
		return fmt.Errorf("profile name contains a control character")
	}
	return nil
}
