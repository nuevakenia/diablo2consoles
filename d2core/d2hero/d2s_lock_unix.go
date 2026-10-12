//go:build !windows
// +build !windows

package d2hero

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// lockWait is how long a writer waits for another process that is saving the same hero.
const lockWait = 10 * time.Second

// lockD2S takes an exclusive advisory lock for the saves of one .d2s (a hidden
// ".<name>.lock" next to it) and returns the function that releases it. Two game
// windows that autosave the same hero take turns; the lock dies with the process,
// so a crash never leaves one behind.
func lockD2S(path string) (func(), error) {
	lockPath := filepath.Join(filepath.Dir(filepath.Clean(path)), "."+filepath.Base(path)+".lock")

	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, writefilePermission)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(lockWait)

	for {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() {
				_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
				_ = file.Close()
			}, nil
		}

		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = file.Close()
			return nil, err
		}

		if time.Now().After(deadline) {
			_ = file.Close()
			return nil, ErrSaveLocked
		}

		time.Sleep(20 * time.Millisecond)
	}
}
