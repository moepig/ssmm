package sshconfig

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/uncho/ssmm/internal/app"
	"github.com/uncho/ssmm/internal/openssh"
	"github.com/uncho/ssmm/internal/target"
)

type Profile struct {
	Name         string
	User         string
	IdentityFile string
	Port         int
	Integration  bool
}

func RenderManaged(profiles []Profile, ssmmExecutable string) ([]byte, error) {
	if err := validateExecutable(ssmmExecutable); err != nil {
		return nil, err
	}
	items := append([]Profile(nil), profiles...)
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	var b strings.Builder
	b.WriteString("# Managed by ssmm. Do not edit.\n")
	for _, profile := range items {
		if !profile.Integration {
			continue
		}
		if err := target.ValidateSSHLabel(profile.Name); err != nil {
			return nil, err
		}
		options := app.SSHOptions{User: profile.User, IdentityFile: profile.IdentityFile, Port: profile.Port}
		if err := options.Validate(); err != nil {
			return nil, err
		}
		b.WriteString("Host *.")
		b.WriteString(profile.Name)
		b.WriteString(".ssmm\n")
		b.WriteString("    HostName %h\n")
		b.WriteString("    CanonicalizeHostname no\n")
		if profile.User != "" {
			value, err := openssh.QuoteConfigValue(profile.User)
			if err != nil {
				return nil, err
			}
			b.WriteString("    User ")
			b.WriteString(configValueForRender(value, profile.User))
			b.WriteByte('\n')
		}
		if profile.IdentityFile != "" {
			value, err := openssh.QuoteIdentityFile(profile.IdentityFile)
			if err != nil {
				return nil, err
			}
			b.WriteString("    IdentityFile ")
			b.WriteString(configValueForRender(value, profile.IdentityFile))
			b.WriteByte('\n')
		}
		if profile.Port != 0 {
			b.WriteString(fmt.Sprintf("    Port %d\n", profile.Port))
		}
		proxy, err := openssh.StandardProxyCommand(ssmmExecutable, profile.Name)
		if err != nil {
			return nil, err
		}
		b.WriteString("    ProxyCommand ")
		b.WriteString(proxy)
		b.WriteByte('\n')
		b.WriteString("    ControlPath none\n\n")
	}
	return []byte(b.String()), nil
}

func configValueForRender(quoted, original string) string {
	if strings.IndexFunc(original, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\\' || r == '"' || r == '\'' || r == '#'
	}) < 0 {
		return strings.Trim(quoted, `"`)
	}
	return quoted
}

func validateExecutable(value string) error {
	if value == "" || !filepath.IsAbs(value) {
		return fmt.Errorf("ssmm executable must be an absolute path")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("ssmm executable contains a control character")
		}
	}
	return nil
}
