package wrappers

import (
	"io"
)

type WriterWrapper struct {
	isClosed bool
	wrapped  io.Writer
}

func NewWriterWrapper(wraps io.Writer) *WriterWrapper {
	return &WriterWrapper{
		isClosed: false,
		wrapped:  wraps,
	}
}

func (r *WriterWrapper) Close() error {
	r.isClosed = true
	return nil
}

func (r *WriterWrapper) Write(p []byte) (n int, err error) {
	if r.isClosed {
		return 0, ErrClosed
	}
	return r.wrapped.Write(p)
}
