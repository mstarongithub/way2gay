package wrappers

import (
	"errors"
	"io"
)

var ErrClosed = errors.New("closed")

type ReaderWrapper struct {
	isClosed bool
	wrapped  io.Reader
}

// Close implements repl.ReadCloser.
func (r *ReaderWrapper) Close() error {
	r.isClosed = true
	return nil
}

// Read implements repl.ReadCloser.
func (r *ReaderWrapper) Read(p []byte) (n int, err error) {
	if r.isClosed {
		return 0, ErrClosed
	}
	return r.wrapped.Read(p)
}

func NewReaderWrapper(wraps io.Reader) *ReaderWrapper {
	return &ReaderWrapper{
		isClosed: false,
		wrapped:  wraps,
	}
}
