package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Paths struct {
	ConfigFile     string
	SSHManagedFile string
	SSHConfigFile  string
	LockFile       string
}

func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return PathsForHome(home), nil
}

func PathsForHome(home string) Paths {
	sshDir := filepath.Join(home, ".ssh")
	return Paths{
		ConfigFile:     filepath.Join(home, ".config", "ssmm", "config.toml"),
		SSHManagedFile: filepath.Join(sshDir, "ssmm", "config"),
		SSHConfigFile:  filepath.Join(sshDir, "config"),
		LockFile:       filepath.Join(home, ".config", "ssmm", "init.lock"),
	}
}

func EnsureParent(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	return nil
}
