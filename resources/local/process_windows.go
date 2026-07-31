package local

import "os"

// processExists reports whether a pid is a live process.
//
// Windows has no signals to probe with, but it does something Unix does not:
// os.FindProcess opens a handle and fails when there is nothing to open, so the
// lookup is the check.
func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = proc.Release()
	return true
}
