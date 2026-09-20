package sshconfig

import (
	"fmt"
	"sort"
	"strings"
)

type Profile struct {
	Name         string
	User         string
	IdentityFile string
	Port         int
	Integration  bool
}

func RenderManaged(profiles []Profile, ssmmExecutable string) ([]byte, error) {
	if ssmmExecutable == "" {
		return nil, fmt.Errorf("ssmm executable is required")
	}
	items := append([]Profile(nil), profiles...)
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	var b strings.Builder
	b.WriteString("# Managed by ssmm. Do not edit.\n")
	for _, profile := range items {
		if !profile.Integration {
			continue
		}
		if err := validateLabel(profile.Name); err != nil {
			return nil, err
		}
		b.WriteString("Host *.")
		b.WriteString(profile.Name)
		b.WriteString(".ssmm\n")
		b.WriteString("    HostName %h\n")
		b.WriteString("    CanonicalizeHostname no\n")
		if profile.User != "" {
			b.WriteString("    User ")
			b.WriteString(sshValue(profile.User))
			b.WriteByte('\n')
		}
		if profile.IdentityFile != "" {
			b.WriteString("    IdentityFile ")
			b.WriteString(sshValue(escapePercent(profile.IdentityFile)))
			b.WriteByte('\n')
		}
		if profile.Port != 0 {
			b.WriteString(fmt.Sprintf("    Port %d\n", profile.Port))
		}
		b.WriteString("    ProxyCommand ")
		b.WriteString(shellQuote(ssmmExecutable))
		b.WriteString(" proxy '%h' --ssmm-profile ")
		b.WriteString(shellQuote(profile.Name))
		b.WriteString(" --port '%p'\n")
		b.WriteString("    ControlPath none\n\n")
	}
	return []byte(b.String()), nil
}

func escapePercent(value string) string { return strings.ReplaceAll(value, "%", "%%") }
func sshValue(value string) string {
	if strings.ContainsAny(value, " \t'\"$\\") {
		value = strings.ReplaceAll(value, `\`, `\\`)
		value = strings.ReplaceAll(value, `"`, `\"`)
		return `"` + value + `"`
	}
	return value
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func validateLabel(value string) error {
	if value == "" || len(value) > 63 || strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") {
		return fmt.Errorf("invalid SSH profile label %q", value)
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' {
			return fmt.Errorf("invalid SSH profile label %q", value)
		}
	}
	return nil
}
