//go:build !windows

package d2app

import (
	"syscall"
	"time"
)

// processCPU is the user+system CPU time the process has used so far; unlike wall-clock frame
// times it is not inflated when other programs compete for the machine.
func processCPU() time.Duration {
	var ru syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &ru) != nil {
		return 0
	}

	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
