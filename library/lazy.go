package library

import (
	"fmt"
	"io"
)

type LazyOpener func() (io.ReadCloser, error)

type LazyReader struct {
	Opener     LazyOpener
	readCloser io.ReadCloser
}

func NewLazyReader(opener LazyOpener) *LazyReader {
	return &LazyReader{
		Opener: opener,
	}
}

func (self *LazyReader) Read(b []byte) (int, error) {
	if self.readCloser == nil {
		if self.Opener != nil {
			if rc, err := self.Opener(); err == nil {
				self.readCloser = rc
			} else {
				return 0, err
			}
		} else {
			return 0, fmt.Errorf("No opener specified")
		}
	}

	return self.readCloser.Read(b)
}

func (self *LazyReader) Close() error {
	if self.readCloser != nil {
		return self.readCloser.Close()
	}

	return nil
}

func (self *LazyReader) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := self.readCloser.(io.Seeker); ok {
		return seeker.Seek(offset, whence)
	} else {
		return 0, fmt.Errorf("source not seekable")
	}
}
