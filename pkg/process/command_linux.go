//go:build !windows

package process

import (
	"os"
	"os/exec"
	"syscall"
)

func configureChildProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
}

func platformBinaryPath(path string) string { return path }

type childProcessTree struct{}

func startChildProcess(cmd *exec.Cmd) (childProcessTree, error) {
	return childProcessTree{}, cmd.Start()
}
func closeChildProcessTree(childProcessTree) error { return nil }

func signalChildProcess(_ childProcessTree, process *os.Process, signal syscall.Signal) error {
	return process.Signal(signal)
}
func interruptChildProcess(_ childProcessTree, process *os.Process) error {
	return process.Signal(syscall.SIGINT)
}
func killChildProcess(_ childProcessTree, process *os.Process) error {
	return process.Signal(syscall.SIGKILL)
}
