package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

func Encode(settings Settings) ([]byte, error) {
	if settings.Profiles == nil {
		settings.Profiles = map[string]Profile{}
	}
	var buf []byte
	var err error
	buf, err = toml.Marshal(settings)
	if err != nil {
		return nil, fmt.Errorf("encode config: %w", err)
	}
	return buf, nil
}

func WriteAtomic(path string, data []byte, mode os.FileMode) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			return resolveErr
		}
		path = resolved
	}
	if err := EnsureParent(path); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ssmm-config-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func Save(path string, settings Settings) error {
	if err := settings.Validate(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := Encode(settings)
	if err != nil {
		return err
	}
	return WriteAtomic(path, data, 0o600)
}

func syncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
