//go:build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

type fileLock struct {
	f *os.File
}

func acquireLock(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("flock: %w", err)
	}

	return &fileLock{f: f}, nil
}

func releaseLock(lock *fileLock) {
	if lock == nil || lock.f == nil {
		return
	}
	syscall.Flock(int(lock.f.Fd()), syscall.LOCK_UN)
	lock.f.Close()
}
