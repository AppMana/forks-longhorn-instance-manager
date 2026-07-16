//go:build !windows

package sparse

import (
	"os"
	"syscall"
)

func openDirectFile(name string, flag int, perm os.FileMode, create bool) (*os.File, error) {
	file, err := os.OpenFile(name, syscall.O_DIRECT|flag, perm)
	if err != nil && create {
		file, err = os.OpenFile(name, os.O_CREATE|syscall.O_DIRECT|flag, perm)
	}
	return file, err
}
