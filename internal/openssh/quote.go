package openssh

import (
	"fmt"
	"strings"
)

func EncodePercent(value string) (string, error) {
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf("value contains a control character")
	}
	return strings.ReplaceAll(value, "%", "%%"), nil
}

func ShellQuote(value string) (string, error) {
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", fmt.Errorf("value contains a control character")
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'", nil
}

func ProxyCommand(executable, instanceID, region string, port int, profile, source string, standardTokens bool) (string, error) {
	if executable == "" || instanceID == "" || region == "" {
		return "", fmt.Errorf("proxy command requires executable, ID, and region")
	}
	if port < 1 || port > 65535 {
		return "", fmt.Errorf("port is out of range")
	}
	values := []string{executable, "proxy", instanceID, "--region", region, "--port", fmt.Sprint(port), "--profile", profile, "--internal-profile-source", source}
	if standardTokens {
		values = []string{executable, "proxy", "%h", "--port", "%p", "--ssmm-profile", profile}
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		if !standardTokens {
			var err error
			value, err = EncodePercent(value)
			if err != nil {
				return "", err
			}
		}
		quoted, err := ShellQuote(value)
		if err != nil {
			return "", err
		}
		parts = append(parts, quoted)
	}
	return strings.Join(parts, " "), nil
}
