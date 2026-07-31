package local

import (
	"os"
	"os/exec"
	"testing"
)

// The guard on every mutation rests on this: a false "not running" lets a write
// through while the client is up, and Steam discards it on exit. It used to ask
// /proc, which answers only on Linux.
func TestProcessExists(t *testing.T) {
	if !processExists(os.Getpid()) {
		t.Error("this process is running and was not found")
	}

	// A pid that has exited is the case a stale steam.pid produces. Spawning
	// one and reaping it is the only way to be sure the number is free.
	cmd := exec.Command(os.Args[0], "-test.run=TestProcessExistsHelperNoop")
	cmd.Env = append(os.Environ(), "GO_TEST_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Wait()

	if processExists(pid) {
		t.Errorf("pid %d has exited and was reported as running", pid)
	}
	if processExists(-1) || processExists(0) {
		t.Error("a nonsense pid must not read as running")
	}
}

func TestProcessExistsHelperNoop(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER") == "" {
		t.Skip("helper for TestProcessExists")
	}
}
