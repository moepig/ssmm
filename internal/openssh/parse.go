package openssh

import (
	"fmt"
	"strings"
)

type Endpoint struct {
	Local  bool
	User   string
	Target string
	Path   string
}

type TransferDirection string

const (
	Send    TransferDirection = "send"
	Receive TransferDirection = "receive"
)

type Transfer struct {
	Direction TransferDirection
	Local     []string
	Remote    []Endpoint
	User      string
	Target    string
}

func ParseEndpoint(value string) (Endpoint, error) {
	if value == "" {
		return Endpoint{}, fmt.Errorf("empty scp operand")
	}
	if strings.HasPrefix(value, "scp://") || strings.HasPrefix(value, "[") {
		return Endpoint{}, fmt.Errorf("URI and IPv6 scp operands are unsupported")
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") {
		return Endpoint{Local: true, Path: value}, nil
	}
	idx := strings.IndexByte(value, ':')
	if idx < 0 {
		return Endpoint{Local: true, Path: value}, nil
	}
	left, path := value[:idx], value[idx+1:]
	if left == "" || path == "" {
		return Endpoint{}, fmt.Errorf("remote operand requires TARGET:PATH")
	}
	user, target := "", left
	if at := strings.LastIndexByte(left, '@'); at >= 0 {
		user, target = left[:at], left[at+1:]
		if user == "" || target == "" {
			return Endpoint{}, fmt.Errorf("invalid remote user and target")
		}
	}
	if strings.ContainsAny(target, "\x00\r\n /\\") {
		return Endpoint{}, fmt.Errorf("invalid remote target")
	}
	return Endpoint{User: user, Target: target, Path: path}, nil
}

func ParseTransfer(args []string) (Transfer, error) {
	if len(args) < 2 {
		return Transfer{}, fmt.Errorf("scp requires at least a source and destination")
	}
	endpoints := make([]Endpoint, len(args))
	for i, arg := range args {
		var err error
		endpoints[i], err = ParseEndpoint(arg)
		if err != nil {
			return Transfer{}, err
		}
	}
	dest := endpoints[len(endpoints)-1]
	sources := endpoints[:len(endpoints)-1]
	if dest.Local {
		for _, source := range sources {
			if source.Local {
				return Transfer{}, fmt.Errorf("transfer requires a remote source or destination")
			}
		}
		for _, source := range sources {
			if source.Target != sources[0].Target || source.User != sources[0].User {
				return Transfer{}, fmt.Errorf("all remote sources must use one target and user")
			}
		}
		return Transfer{Direction: Receive, Local: []string{dest.Path}, Remote: append([]Endpoint(nil), sources...), User: sources[0].User, Target: sources[0].Target}, nil
	}
	for _, source := range sources {
		if !source.Local {
			return Transfer{}, fmt.Errorf("remote-to-remote transfer is unsupported")
		}
	}
	return Transfer{Direction: Send, Local: localPaths(sources), Remote: []Endpoint{dest}, User: dest.User, Target: dest.Target}, nil
}

func localPaths(endpoints []Endpoint) []string {
	out := make([]string, len(endpoints))
	for i, e := range endpoints {
		out[i] = e.Path
	}
	return out
}
