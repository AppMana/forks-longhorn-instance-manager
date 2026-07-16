//go:build windows

package cmd

import (
	"context"

	"github.com/longhorn/longhorn-instance-manager/pkg/process"
	windowstarget "github.com/longhorn/longhorn-instance-manager/pkg/target/windows"
)

func newProcessLifecycle(ctx context.Context) (process.Lifecycle, error) {
	return windowstarget.NewManager(ctx, ":3260")
}
