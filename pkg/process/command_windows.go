//go:build windows

package process

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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

// Windows does not implement POSIX signals for os.Process. The target backend
// has already switched and drained before this is called during replacement,
// so terminating the old process preserves the same externally visible I/O
// ordering as Linux's SIGHUP path.
func signalChildProcess(process *os.Process, _ syscall.Signal) error { return process.Kill() }
func interruptChildProcess(process *os.Process) error                { return process.Kill() }
func killChildProcess(process *os.Process) error                     { return process.Kill() }
