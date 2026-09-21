package app

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/uncho/ssmm/internal/target"
)

type SettingsSnapshot struct {
	ConfigPath string
	ConfigDir  string
	Profiles   map[string]ProfileSettings
}

type ProfileSettings struct {
	AWSProfile         string
	Regions            *[]string
	SSH                SSHOptions
	StoredIdentityFile string
	Integration        bool
}

type SSHOptions struct {
	User         string
	IdentityFile string
	Port         int
}

func (o SSHOptions) Validate() error {
	if o.User != "" {
		if o.User[0] == '-' {
			return fmt.Errorf("SSH user %q must not start with '-'", o.User)
		}
		for _, r := range o.User {
			if unicode.IsControl(r) || unicode.IsSpace(r) || strings.ContainsRune("@:/\\", r) {
				return fmt.Errorf("SSH user %q contains an invalid character", o.User)
			}
		}
	}
	if o.IdentityFile != "" {
		for _, r := range o.IdentityFile {
			if unicode.IsControl(r) {
				return fmt.Errorf("identity file %q contains a control character", o.IdentityFile)
			}
		}
		if strings.Contains(o.IdentityFile, "${") {
			return fmt.Errorf("identity file %q contains unsupported ${...}", o.IdentityFile)
		}
	}
	if o.Port < 0 || o.Port > 65535 {
		return fmt.Errorf("SSH port %d is out of range", o.Port)
	}
	return nil
}

func (s SettingsSnapshot) Profile(name string) ProfileSettings {
	if p, ok := s.Profiles[name]; ok {
		return p
	}
	return ProfileSettings{}
}

type SettingsDraft struct {
	UpdateSSH bool
	Snapshot  SettingsSnapshot
	Original  map[string]FileState
}
type SettingsUpdate struct {
	UpdateSSH bool
	Snapshot  SettingsSnapshot
	Original  map[string]FileState
}
type FileState struct {
	Path     string
	Exists   bool
	Data     []byte
	Mode     uint32
	Resolved string
}
type SaveReport struct{ Files []SaveFile }
type SaveFile struct {
	Path   string
	Status string
	Err    error
}

type SessionRequest struct {
	Target target.ResolvedTarget
	Proxy  bool
	Port   int
}
type SSHRequest struct {
	Target         target.ResolvedTarget
	Options        SSHOptions
	Executable     string
	SsmmExecutable string
	ExtraArgs      []string
}
type SCPRequest struct {
	Target         target.ResolvedTarget
	Options        SSHOptions
	Executable     string
	SsmmExecutable string
	Transfer       SCPTransfer
	Recursive      bool
}

type SCPDirection string

const (
	SCPSend    SCPDirection = "send"
	SCPReceive SCPDirection = "receive"
)

type SCPRemote struct {
	User   string
	Target string
	Path   string
}

type SCPTransfer struct {
	Direction SCPDirection
	Local     []string
	Remote    []SCPRemote
	User      string
	Target    string
}

// コマンドライン、ssmm 設定、環境変数、既定値の順に AWS プロファイルを選択する。
func (s SettingsSnapshot) AWSSelection(name string, fallback target.ProfileSelection) target.ProfileSelection {
	if fallback.Source == target.ProfileFlag {
		return fallback
	}
	if profile := s.Profile(name).AWSProfile; profile != "" {
		return target.ProfileSelection{Name: profile, Source: target.ProfileConfig}
	}
	if fallback.Name == "" {
		return target.ProfileSelection{Name: "default", Source: target.ProfileDefault}
	}
	return fallback
}
