package sshconfig

import "strings"

func EnsureInclude(existing []byte, managedPath string) []byte {
	line := "Include " + includeValue(managedPath)
	lines := strings.Split(strings.ReplaceAll(string(existing), "\r\n", "\n"), "\n")
	kept := make([]string, 0, len(lines)+1)
	found := false
	for _, current := range lines {
		trimmed := strings.TrimSpace(current)
		if isIncludeFor(trimmed, managedPath) {
			if found {
				continue
			}
			found = true
		}
		kept = append(kept, current)
	}
	if !found {
		kept = append([]string{line}, kept...)
	}
	return []byte(strings.Join(kept, "\n"))
}

func RemoveInclude(existing []byte, managedPath string) []byte {
	lines := strings.Split(strings.ReplaceAll(string(existing), "\r\n", "\n"), "\n")
	kept := make([]string, 0, len(lines))
	for _, current := range lines {
		trimmed := strings.TrimSpace(current)
		if isIncludeFor(trimmed, managedPath) {
			continue
		}
		kept = append(kept, current)
	}
	return []byte(strings.Join(kept, "\n"))
}

func includeValue(path string) string {
	if strings.ContainsAny(path, " \t") {
		return `"` + strings.ReplaceAll(path, `"`, `\"`) + `"`
	}
	return path
}

func isIncludeFor(line, path string) bool {
	if !strings.HasPrefix(line, "Include ") {
		return false
	}
	value := strings.TrimSpace(strings.TrimPrefix(line, "Include "))
	value = strings.Trim(value, `"`)
	return value == path
}
