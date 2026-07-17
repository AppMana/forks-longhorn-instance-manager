//go:build windows

package process

import (
	"io"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestBinaryCommandKillTerminatesDescendants(t *testing.T) {
	pidFile := t.TempDir() + `\grandchild.pid`
	command, err := NewBinaryCommand(os.Args[0], "-test.run=TestProcessTreeHelper")
	if err != nil {
		t.Fatal(err)
	}
	command.Cmd.Env = append(os.Environ(),
		"LONGHORN_JOB_TEST_ROLE=parent",
		"LONGHORN_JOB_TEST_PID_FILE="+pidFile,
	)
	command.SetOutput(io.Discard)
	runCh := make(chan error, 1)
	go func() {
		runCh <- command.Run()
	}()
	t.Cleanup(func() {
		if command.IsRunning() {
			command.Kill()
		}
	})

	grandchildPID := waitForGrandchildPID(t, pidFile)
	if !processIsActive(grandchildPID) {
		t.Fatalf("grandchild process %d was not running before job termination", grandchildPID)
	}

	command.Kill()
	select {
	case <-runCh:
	case <-time.After(10 * time.Second):
		t.Fatal("parent process did not exit after job termination")
	}

	deadline := time.Now().Add(10 * time.Second)
	for processIsActive(grandchildPID) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if processIsActive(grandchildPID) {
		t.Fatalf("grandchild process %d survived job termination", grandchildPID)
	}
}

func TestProcessTreeHelper(t *testing.T) {
	switch os.Getenv("LONGHORN_JOB_TEST_ROLE") {
	case "":
		return
	case "grandchild":
		for {
			time.Sleep(time.Hour)
		}
	case "parent":
		child := exec.Command(os.Args[0], "-test.run=TestProcessTreeHelper")
		child.Env = append(os.Environ(), "LONGHORN_JOB_TEST_ROLE=grandchild")
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		pidFile := os.Getenv("LONGHORN_JOB_TEST_PID_FILE")
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
			os.Exit(3)
		}
		for {
			time.Sleep(time.Hour)
		}
	default:
		os.Exit(4)
	}
}

func waitForGrandchildPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		content, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(string(content))
			if err != nil {
				t.Fatalf("invalid grandchild pid %q: %v", content, err)
			}
			return pid
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("timed out waiting for grandchild pid")
	return 0
}

func processIsActive(pid int) bool {
	const stillActive = 259
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	var exitCode uint32
	if err := windows.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false
	}
	return exitCode == stillActive
}
