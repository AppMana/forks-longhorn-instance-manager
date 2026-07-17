package replica

import "github.com/longhorn/longhorn-engine/pkg/types"

const MaxExtentsBuffer = 1024

func LoadDiffDiskLocationList(diffDisk *diffDisk, disk types.DiffDisk, currentFileIndex byte) error {
	fd := disk.Fd()

	start := uint64(0)
	end := uint64(len(diffDisk.location)) * uint64(diffDisk.sectorSize)
	for {
		extents, more, err := queryAllocatedExtents(fd, start, end-start, MaxExtentsBuffer)
		if err != nil {
			return err
		}

		if len(extents) == 0 {
			return nil
		}

		for _, extent := range extents {
			for i := int64(0); i < int64(extent.Length); i += diffDisk.sectorSize {
				diffDisk.location[(int64(extent.Logical)+i)/diffDisk.sectorSize] = currentFileIndex
			}
		}

		if !more {
			return nil
		}
		start = extents[len(extents)-1].Logical + extents[len(extents)-1].Length
	}
}
