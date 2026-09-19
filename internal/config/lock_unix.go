//go:build unix

package config

import (
	"os"
	"path/filepath"
	"syscall"
)

func acquireLock(path string) (func(), error) {
	if err := EnsureParent(path); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
		_ = syncDir(filepath.Dir(path))
	}, nil
}
