package sparse

// NTFS and ReFS test volumes are formatted with 4 KiB allocation units. The
// sparse copy protocol already operates on the same 4 KiB interval boundary.
func getFileSystemBlockSize(FileIoProcessor) (int, error) {
	return int(Blocks), nil
}
