package execplan

import (
	"fmt"
	"os"
	"strings"
)

type Mode string

const (
	Foreground  Mode = "foreground"
	ProxyStream Mode = "proxy-stream"
)

type EnvironmentPolicy struct {
	UnsetPager        bool
	DisableAutoPrompt bool
	Profile           string
	ProfileSource     string
}

type ProcessSpec struct {
	Executable        string
	Args              []string
	EnvironmentPolicy EnvironmentPolicy
	Mode              Mode
}

type Streams struct {
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

func (s ProcessSpec) Validate() error {
	if s.Executable == "" || !strings.HasPrefix(s.Executable, "/") {
		return fmt.Errorf("executable must be an absolute path")
	}
	if strings.ContainsAny(s.Executable, "\x00\r\n") {
		return fmt.Errorf("executable contains a control character")
	}
	for _, arg := range s.Args {
		if strings.ContainsRune(arg, 0) {
			return fmt.Errorf("argument contains NUL")
		}
	}
	if s.Mode != Foreground && s.Mode != ProxyStream {
		return fmt.Errorf("invalid process mode %q", s.Mode)
	}
	return nil
}

func (s ProcessSpec) Clone() ProcessSpec {
	s.Args = append([]string(nil), s.Args...)
	return s
}
