//go:build !windows

package cmd

import (
	"context"

	"github.com/longhorn/longhorn-instance-manager/pkg/process"
)

func newProcessLifecycle(context.Context) (process.Lifecycle, error) {
	return nil, nil
}
