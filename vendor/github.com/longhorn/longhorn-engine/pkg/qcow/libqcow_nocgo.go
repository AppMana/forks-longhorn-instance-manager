//go:build !windows && !cgo

package qcow

import (
	"fmt"
	"io"
)

// Qcow keeps client-only non-CGO builds usable. Linux engine builds use
// libqcow.go; attempting to open QCOW data without CGO fails explicitly.
type Qcow struct{}

func Open(path string) (*Qcow, error) {
	return nil, fmt.Errorf("QCOW backing files require CGO and libqcow: %s", path)
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

func (q *Qcow) Close() error { return nil }

func (q Qcow) Size() (int64, error) {
	return 0, fmt.Errorf("QCOW size is unavailable")
}

func (q *Qcow) Fd() uintptr { return 0 }
