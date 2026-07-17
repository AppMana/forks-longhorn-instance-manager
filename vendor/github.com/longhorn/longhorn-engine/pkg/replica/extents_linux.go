//go:build !windows

package replica

import "github.com/rancher/go-fibmap"

type allocatedExtent struct {
	Logical uint64
	Length  uint64
}

func queryAllocatedExtents(fd uintptr, start, length uint64, maxExtents uint32) ([]allocatedExtent, bool, error) {
	extents, errno := fibmap.Fiemap(fd, start, length, maxExtents)
	if errno != 0 {
		return nil, false, errno
	}

	result := make([]allocatedExtent, 0, len(extents))
	for _, extent := range extents {
		result = append(result, allocatedExtent{Logical: extent.Logical, Length: extent.Length})
		if extent.Flags&fibmap.FIEMAP_EXTENT_LAST != 0 {
			return result, false, nil
		}
	}
	return result, len(result) == int(maxExtents), nil
}

func extentQueriesSupported(fd uintptr, size uint64) error {
	_, _, err := queryAllocatedExtents(fd, 0, size, 1)
	return err
}
