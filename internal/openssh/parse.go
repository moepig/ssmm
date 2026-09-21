package openssh

import (
	"fmt"
	"strings"

	"github.com/uncho/ssmm/internal/app"
)

type endpoint struct {
	Local  bool
	User   string
	Target string
	Path   string
}

func parseEndpoint(value string) (endpoint, error) {
	if value == "" {
		return endpoint{}, fmt.Errorf("empty scp operand")
	}
	if strings.HasPrefix(value, "scp://") || strings.HasPrefix(value, "[") {
		return endpoint{}, fmt.Errorf("URI and IPv6 scp operands are unsupported")
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") {
		return endpoint{Local: true, Path: value}, nil
	}
	idx := strings.IndexByte(value, ':')
	if idx < 0 {
		return endpoint{Local: true, Path: value}, nil
	}
	left, path := value[:idx], value[idx+1:]
	if left == "" || path == "" {
		return endpoint{}, fmt.Errorf("remote operand requires TARGET:PATH")
	}
	user, target := "", left
	if at := strings.LastIndexByte(left, '@'); at >= 0 {
		user, target = left[:at], left[at+1:]
		if user == "" || target == "" {
			return endpoint{}, fmt.Errorf("invalid remote user and target")
		}
		if err := (app.SSHOptions{User: user}).Validate(); err != nil {
			return endpoint{}, err
		}
	}
	if strings.ContainsAny(target, "\x00\r\n /\\") {
		return endpoint{}, fmt.Errorf("invalid remote target")
	}
	return endpoint{User: user, Target: target, Path: path}, nil
}

func ParseTransfer(args []string) (app.SCPTransfer, error) {
	if len(args) < 2 {
		return app.SCPTransfer{}, fmt.Errorf("scp requires at least a source and destination")
	}
	endpoints := make([]endpoint, len(args))
	for i, arg := range args {
		var err error
		endpoints[i], err = parseEndpoint(arg)
		if err != nil {
			return app.SCPTransfer{}, err
		}
	}
	dest := endpoints[len(endpoints)-1]
	sources := endpoints[:len(endpoints)-1]
	if dest.Local {
		for _, source := range sources {
			if source.Local {
				return app.SCPTransfer{}, fmt.Errorf("transfer requires a remote source or destination")
			}
		}
		for _, source := range sources {
			if source.Target != sources[0].Target || source.User != sources[0].User {
				return app.SCPTransfer{}, fmt.Errorf("all remote sources must use one target and user")
			}
		}
		remote := make([]app.SCPRemote, len(sources))
		for i, source := range sources {
			remote[i] = app.SCPRemote{User: source.User, Target: source.Target, Path: source.Path}
		}
		return app.SCPTransfer{Direction: app.SCPReceive, Local: []string{dest.Path}, Remote: remote, User: sources[0].User, Target: sources[0].Target}, nil
	}
	for _, source := range sources {
		if !source.Local {
			return app.SCPTransfer{}, fmt.Errorf("remote-to-remote transfer is unsupported")
		}
	}
	return app.SCPTransfer{Direction: app.SCPSend, Local: localPaths(sources), Remote: []app.SCPRemote{{User: dest.User, Target: dest.Target, Path: dest.Path}}, User: dest.User, Target: dest.Target}, nil
}

func localPaths(endpoints []endpoint) []string {
	out := make([]string, len(endpoints))
	for i, e := range endpoints {
		out[i] = e.Path
	}
	return out
}
