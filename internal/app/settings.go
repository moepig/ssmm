package app

import "github.com/uncho/ssmm/internal/target"

type SettingsSnapshot struct {
	ConfigPath string
	ConfigDir  string
	Profiles   map[string]ProfileSettings
}

type ProfileSettings struct {
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

func (s SettingsSnapshot) Profile(name string) ProfileSettings {
	if p, ok := s.Profiles[name]; ok {
		return p
	}
	return ProfileSettings{}
}

type SettingsDraft struct {
	Snapshot SettingsSnapshot
	Original map[string]FileState
}
type SettingsUpdate struct {
	Snapshot SettingsSnapshot
	Original map[string]FileState
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
