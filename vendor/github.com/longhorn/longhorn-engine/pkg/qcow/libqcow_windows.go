package qcow

import (
	"fmt"
	"io"
)

// Qcow is retained on Windows so the common backing-file code can compile.
// libqcow is a C library without a supported Windows build in Longhorn. Failing
// Open explicitly is preferable to silently treating a QCOW image as raw.
type Qcow struct{}

func Open(path string) (*Qcow, error) {
	return nil, fmt.Errorf("QCOW backing files are not supported on Windows: %s", path)
}

func (q *Qcow) WriteAt([]byte, int64) (int, error) {
	return 0, fmt.Errorf("QCOW writes are unsupported")
}

func (q *Qcow) ReadAt([]byte, int64) (int, error) {
	return 0, io.EOF
}

func (q *Qcow) UnmapAt(uint32, int64) (int, error) {
	return 0, fmt.Errorf("QCOW unmap is unsupported")
}

func (q *Qcow) Close() error {
	return nil
}

func (q Qcow) Size() (int64, error) {
	return 0, fmt.Errorf("QCOW size is unavailable")
}

func (q *Qcow) Fd() uintptr {
	return 0
}
