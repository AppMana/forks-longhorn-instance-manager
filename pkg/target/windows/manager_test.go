//go:build windows

package windows

import (
	"testing"

	"github.com/gostor/gotgt/pkg/scsi"
)

func TestManagerRegistersNullBackingStoreForLUN0(t *testing.T) {
	// gotgt constructs a synthetic LUN 0 for every target. Its null backing
	// store is a plugin registered by package init; without the side-effect
	// import in manager.go, NewLUN0 dereferences a nil backing store and takes
	// down the entire Windows instance-manager process.
	lun := scsi.NewLUN0()
	if lun == nil || lun.Storage == nil {
		t.Fatal("gotgt synthetic LUN 0 has no null backing store")
	}
}
