package openssh

import (
	"fmt"
	"strings"
	"unicode"
)

func QuoteConfigValue(value string) (string, error) {
	if err := rejectControl(value); err != nil {
		return "", err
	}
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`, nil
}

func QuoteIdentityFile(value string) (string, error) {
	if strings.Contains(value, "${") {
		return "", fmt.Errorf("identity file contains unsupported ${...}")
	}
	encoded, err := EncodePercent(value)
	if err != nil {
		return "", err
	}
	return QuoteConfigValue(encoded)
}

func EncodePercent(value string) (string, error) {
	if err := rejectControl(value); err != nil {
		return "", err
	}
	return strings.ReplaceAll(value, "%", "%%"), nil
}

func ShellQuote(value string) (string, error) {
	if err := rejectControl(value); err != nil {
		return "", err
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'", nil
}

func ProxyCommand(executable, instanceID, region string, port int, profile, source string) (string, error) {
	if executable == "" || instanceID == "" || region == "" {
		return "", fmt.Errorf("proxy command requires executable, ID, and region")
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("port is out of range")
	}
	values := []string{executable, "proxy", instanceID, "--region", region, "--port", fmt.Sprint(port), "--profile", profile, "--internal-profile-source", source}
	return fixedProxyCommand(values)
}

func StandardProxyCommand(executable, profile string) (string, error) {
	if executable == "" || profile == "" {
		return "", fmt.Errorf("standard proxy command requires executable and profile")
	}
	values := []string{executable, "proxy", "%h", "--ssmm-profile", profile, "--port", "%p"}
	parts := make([]string, 0, len(values))
	for i, value := range values {
		if i != 2 && i != 6 {
			var err error
			value, err = EncodePercent(value)
			if err != nil {
				return "", err
			}
		}
		if i == 0 || i == 2 || i == 4 || i == 6 {
			quoted, err := ShellQuote(value)
			if err != nil {
				return "", err
			}
			parts = append(parts, quoted)
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, " "), nil
}

func fixedProxyCommand(values []string) (string, error) {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		var err error
		value, err = EncodePercent(value)
		if err != nil {
			return "", err
		}
		quoted, err := ShellQuote(value)
		if err != nil {
			return "", err
		}
		parts = append(parts, quoted)
	}
	return strings.Join(parts, " "), nil
}

func rejectControl(value string) error {
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("value contains a control character")
		}
	}
	return nil
}
