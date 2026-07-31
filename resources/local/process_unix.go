//go:build !windows

package local

import (
	"errors"
	"os"
	"syscall"
)

// processExists reports whether a pid is a live process.
//
// Signal 0 is the portable Unix probe: the kernel runs its permission and
// existence checks and delivers nothing. `/proc` would answer too, and did, but
// only on Linux — and this provider now ships wherever Steam does.
//
// EPERM means the process is there and belongs to somebody else, which is still
// a running client. Treating it as absent would be the dangerous answer.
func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
