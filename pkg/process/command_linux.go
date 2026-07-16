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

func signalChildProcess(process *os.Process, signal syscall.Signal) error {
	return process.Signal(signal)
}
func interruptChildProcess(process *os.Process) error { return process.Signal(syscall.SIGINT) }
func killChildProcess(process *os.Process) error      { return process.Signal(syscall.SIGKILL) }
