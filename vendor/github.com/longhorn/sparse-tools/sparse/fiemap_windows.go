package sparse

import (
	"errors"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	FIEMAP_MAX_OFFSET  = ^uint64(0)
	FIEMAP_EXTENT_LAST = 0x0001

	fsctlQueryAllocatedRanges = 0x000940cf
	fsctlSetZeroData          = 0x000980c8
)

type Extent struct {
	Logical    uint64
	Physical   uint64
	Length     uint64
	Reserved64 [2]uint64
	Flags      uint32
	Reserved   [3]uint32
}

type FiemapFile struct {
	*os.File
}

type fileAllocatedRangeBuffer struct {
	FileOffset int64
	Length     int64
}

func NewFiemapFile(file *os.File) *FiemapFile {
	return &FiemapFile{file}
}

func (f FiemapFile) Fiemap(size uint32) (uint32, []Extent, syscall.Errno) {
	info, err := f.Stat()
	if err != nil {
		return 0, nil, windowsErrno(err)
	}
	return f.FiemapRegion(size, 0, uint64(info.Size()))
}

func (f FiemapFile) FiemapRegion(numExtents uint32, start uint64, length uint64) (uint32, []Extent, syscall.Errno) {
	if numExtents == 0 {
		numExtents = 1024
	}
	input := fileAllocatedRangeBuffer{FileOffset: int64(start), Length: int64(length)}
	output := make([]fileAllocatedRangeBuffer, numExtents)
	var returned uint32
	err := windows.DeviceIoControl(
		windows.Handle(f.Fd()),
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
		return 0, nil, windowsErrno(err)
	}

	count := int(returned / uint32(unsafe.Sizeof(output[0])))
	extents := make([]Extent, 0, count)
	for index, allocated := range output[:count] {
		flags := uint32(0)
		if index == count-1 && !more {
			flags = FIEMAP_EXTENT_LAST
		}
		extents = append(extents, Extent{
			Logical: uint64(allocated.FileOffset),
			Length:  uint64(allocated.Length),
			Flags:   flags,
		})
	}
	return uint32(len(extents)), extents, 0
}

func (f FiemapFile) Fallocate(offset int64, length int64) error {
	end := offset + length
	if end > 0 {
		if info, err := f.Stat(); err == nil && info.Size() < end {
			return f.Truncate(end)
		}
	}
	return nil
}

func (f FiemapFile) PunchHole(offset int64, length int64) error {
	zeroRange := fileAllocatedRangeBuffer{FileOffset: offset, Length: offset + length}
	var returned uint32
	return windows.DeviceIoControl(
		windows.Handle(f.Fd()),
		fsctlSetZeroData,
		(*byte)(unsafe.Pointer(&zeroRange)),
		uint32(unsafe.Sizeof(zeroRange)),
		nil,
		0,
		&returned,
		nil,
	)
}

func windowsErrno(err error) syscall.Errno {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno
	}
	return syscall.EINVAL
}
