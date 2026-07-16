//go:build windows

package meta

// API 3 selects the maintained V1 ProcessManager service. The newer combined
// InstanceService necessarily links the Linux-only V2/SPDK runtime.
func platformInstanceManagerAPIVersion() int { return 3 }
