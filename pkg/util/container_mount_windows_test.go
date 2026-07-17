//go:build windows

package util

import (
	"path/filepath"
	"testing"
)

func TestResolveContainerMountPathForHostProcess(t *testing.T) {
	sandbox := `C:\ProgramData\containerd\root\hpc\sandbox`
	t.Setenv(containerSandboxMountPointEnv, sandbox)

	want := filepath.Join(sandbox, `var\lib\longhorn\tls`)
	if got := ResolveContainerMountPath(`C:\var\lib\longhorn\tls`); got != want {
		t.Fatalf("resolved path %q, want %q", got, want)
	}
	if got := ResolveContainerMountPath(want); got != want {
		t.Fatalf("resolved sandbox path %q, want %q", got, want)
	}
}
