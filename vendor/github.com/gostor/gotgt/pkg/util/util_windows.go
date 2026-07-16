package util

import "os"

func Fdatasync(file *os.File) error {
	return file.Sync()
}

// Windows cache hints are optional; the Longhorn remote backing store does
// not expose a native file handle to advise.
func Fadvise(*os.File, int64, int64, uint32) error {
	return nil
}
