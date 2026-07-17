//go:build windows

package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	winjob "github.com/kolesnikovae/go-winjob"
)

func configureChildProcess(*exec.Cmd) {}

func platformBinaryPath(path string) string { return windowsEngineBinaryPath(path) }

func windowsEngineBinaryPath(path string) string {
	const linuxPrefix = "/var/lib/longhorn/engine-binaries/"
	if !strings.HasPrefix(path, linuxPrefix) {
		return path
	}
	relative := strings.TrimPrefix(path, linuxPrefix)
	parts := strings.Split(relative, "/")
	windowsPath := filepath.Join(append([]string{`C:\var\lib\longhorn\engine-binaries`}, parts...)...)
	if !strings.HasSuffix(strings.ToLower(windowsPath), ".exe") {
		windowsPath += ".exe"
	}
	return windowsPath
}

type childProcessTree struct {
	job *winjob.JobObject
}

func startChildProcess(cmd *exec.Cmd) (childProcessTree, error) {
	// winjob starts the process suspended, assigns it to the nested job, and
	// only then resumes it. This closes the race in which an engine could spawn
	// a sync-agent child before the job assignment completed.
	job, err := winjob.Start(cmd, winjob.WithKillOnJobClose())
	if err != nil {
		return childProcessTree{}, err
	}
	return childProcessTree{job: job}, nil
}

func closeChildProcessTree(processTree childProcessTree) error {
	if processTree.job == nil {
		return nil
	}
	return processTree.job.Close()
}

func terminateChildProcessTree(processTree childProcessTree, process *os.Process) error {
	if processTree.job == nil {
		return process.Kill()
	}
	return processTree.job.Terminate()
}

// Windows does not implement POSIX signals for os.Process. The target backend
// has already switched and drained before this is called during replacement,
// so terminating the old process preserves the same externally visible I/O
// ordering as Linux's SIGHUP path.
func signalChildProcess(processTree childProcessTree, process *os.Process, _ syscall.Signal) error {
	return terminateChildProcessTree(processTree, process)
}
func interruptChildProcess(processTree childProcessTree, process *os.Process) error {
	return terminateChildProcessTree(processTree, process)
}
func killChildProcess(processTree childProcessTree, process *os.Process) error {
	return terminateChildProcessTree(processTree, process)
}
