package sparse

import (
	"os"

	"golang.org/x/sys/windows"
)

const fsctlSetSparse = 0x000900c4

func openDirectFile(name string, flag int, perm os.FileMode, create bool) (*os.File, error) {
	file, err := os.OpenFile(name, flag, perm)
	if err != nil && create {
		file, err = os.OpenFile(name, os.O_CREATE|flag, perm)
	}
	if err != nil {
		return nil, err
	}

	// Windows has no O_DIRECT equivalent in os.OpenFile. Preserve the common
	// replica I/O path with buffered handles, but mark every writable layer
	// sparse so FSCTL_SET_ZERO_DATA can deallocate unmapped ranges on NTFS/ReFS.
	if flag&(os.O_WRONLY|os.O_RDWR) != 0 {
		var returned uint32
		if err := windows.DeviceIoControl(
			windows.Handle(file.Fd()), fsctlSetSparse, nil, 0, nil, 0, &returned, nil,
		); err != nil {
			_ = file.Close()
			return nil, err
		}
	}
	return file, nil
}
