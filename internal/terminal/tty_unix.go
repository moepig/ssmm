//go:build unix

package terminal

import (
	"fmt"
	"os"
)

type TTY struct{ File *os.File }

func Open() (*TTY, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open operation terminal: %w", err)
	}
	return &TTY{File: f}, nil
}

func (t *TTY) Close() error {
	if t == nil || t.File == nil {
		return nil
	}
	return t.File.Close()
}

func Available() bool {
	t, err := Open()
	if err != nil {
		return false
	}
	_ = t.Close()
	return true
}
