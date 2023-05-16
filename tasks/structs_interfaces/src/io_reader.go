package src

import (
	"bytes"
	"io"
	"strings"
)

type Reader interface {
	Read(p []byte) (int, error)
	ReadAll(bufSize int) (string, error)
	BytesRead() int64
}

type CountingToLowerReaderImpl struct {
	Reader         io.Reader
	TotalBytesRead int64
}

func (cr *CountingToLowerReaderImpl) Read(p []byte) (int, error) {
	var (
		n   int
		err error
	)

	if n, err = cr.Reader.Read(p); err == nil || err == io.EOF {
		cr.TotalBytesRead += int64(n)
		copy(p, bytes.ToLower(p))
		return n, err
	}
	return 0, err
}

func (cr *CountingToLowerReaderImpl) BytesRead() int64 {
	return cr.TotalBytesRead
}

func (cr *CountingToLowerReaderImpl) ReadAll(bufSize int) (string, error) {
	var (
		n   int
		err error
		res strings.Builder
		buf = make([]byte, bufSize)
	)

	res.Grow(bufSize)
	for {
		n, err = cr.Read(buf)
		if err != nil {
			break
		}
		res.Write(buf[:n])
	}

	if err == io.EOF {
		return res.String(), nil
	}

	return res.String(), err
}

func NewCountingReader(r io.Reader) *CountingToLowerReaderImpl {
	return &CountingToLowerReaderImpl{
		Reader: r,
	}
}
