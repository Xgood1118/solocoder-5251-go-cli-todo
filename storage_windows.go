//go:build windows

package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

type fileLock struct {
	f *os.File
}

func acquireLock(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	handle := windows.Handle(f.Fd())
	var overlapped windows.Overlapped
	err = windows.LockFileEx(
		handle,
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0,
		1,
		0,
		&overlapped,
	)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("LockFileEx: %w", err)
	}

	return &fileLock{f: f}, nil
}

func releaseLock(lock *fileLock) {
	if lock == nil || lock.f == nil {
		return
	}
	handle := windows.Handle(lock.f.Fd())
	var overlapped windows.Overlapped
	windows.UnlockFileEx(handle, 0, 1, 0, &overlapped)
	lock.f.Close()
}
