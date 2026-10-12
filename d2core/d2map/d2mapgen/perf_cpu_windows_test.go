//go:build windows

package d2mapgen

import (
	"syscall"
	"time"
)

// perfCPU is the user+system CPU time of the process (GetProcessTimes; FILETIME is in 100 ns units).
func perfCPU() time.Duration {
	h, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0
	}

	var creation, exit, kernel, user syscall.Filetime
	if syscall.GetProcessTimes(h, &creation, &exit, &kernel, &user) != nil {
		return 0
	}

	ft := func(f syscall.Filetime) int64 { return int64(f.HighDateTime)<<32 | int64(f.LowDateTime) }

	return time.Duration((ft(kernel) + ft(user)) * 100)
}
