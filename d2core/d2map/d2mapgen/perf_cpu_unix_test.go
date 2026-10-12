//go:build !windows

package d2mapgen

import (
	"syscall"
	"time"
)

// perfCPU is the CPU time of the process: unlike wall-clock time it is not inflated when other
// programs compete for the machine.
func perfCPU() time.Duration {
	var ru syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &ru) != nil {
		return 0
	}

	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
