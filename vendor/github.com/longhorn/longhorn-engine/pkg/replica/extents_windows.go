package replica

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

const fsctlQueryAllocatedRanges = 0x000940cf

type allocatedExtent struct {
	Logical uint64
	Length  uint64
}

type fileAllocatedRangeBuffer struct {
	FileOffset int64
	Length     int64
}

// queryAllocatedExtents is the Windows equivalent of Linux FIEMAP. NTFS and
// ReFS expose the allocated portions of a sparse file through
// FSCTL_QUERY_ALLOCATED_RANGES using the file handle already held by DiffDisk.
func queryAllocatedExtents(fd uintptr, start, length uint64, maxExtents uint32) ([]allocatedExtent, bool, error) {
	input := fileAllocatedRangeBuffer{FileOffset: int64(start), Length: int64(length)}
	output := make([]fileAllocatedRangeBuffer, maxExtents)
	var returned uint32
	err := windows.DeviceIoControl(
		windows.Handle(fd),
		fsctlQueryAllocatedRanges,
		(*byte)(unsafe.Pointer(&input)),
		uint32(unsafe.Sizeof(input)),
		(*byte)(unsafe.Pointer(&output[0])),
		uint32(uintptr(len(output))*unsafe.Sizeof(output[0])),
		&returned,
		nil,
	)
	more := errors.Is(err, windows.ERROR_MORE_DATA)
	if err != nil && !more {
		return nil, false, err
	}

	count := int(returned / uint32(unsafe.Sizeof(output[0])))
	result := make([]allocatedExtent, 0, count)
	for _, extent := range output[:count] {
		result = append(result, allocatedExtent{
			Logical: uint64(extent.FileOffset),
			Length:  uint64(extent.Length),
		})
	}
	return result, more, nil
}

func extentQueriesSupported(fd uintptr, size uint64) error {
	_, _, err := queryAllocatedExtents(fd, 0, size, 1)
	return err
}
