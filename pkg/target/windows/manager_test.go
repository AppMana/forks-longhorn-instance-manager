//go:build windows

package windows

import (
	"testing"

	"github.com/gostor/gotgt/pkg/scsi"
)

func TestGetAdvertisedPortalUsesPodIP(t *testing.T) {
	portal, err := getAdvertisedPortal("[::]:3260", "172.30.0.30")
	if err != nil {
		t.Fatal(err)
	}
	if portal != "172.30.0.30:3260" {
		t.Fatalf("portal = %q, want %q", portal, "172.30.0.30:3260")
	}
}

func TestGetAdvertisedPortalRejectsWildcardWithoutPodIP(t *testing.T) {
	if _, err := getAdvertisedPortal("[::]:3260", ""); err == nil {
		t.Fatal("expected wildcard listener without POD_IP to be rejected")
	}
}

func TestGetAdvertisedPortalAllowsConcreteListener(t *testing.T) {
	portal, err := getAdvertisedPortal("127.0.0.1:3260", "")
	if err != nil {
		t.Fatal(err)
	}
	if portal != "127.0.0.1:3260" {
		t.Fatalf("portal = %q, want %q", portal, "127.0.0.1:3260")
	}
}

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
