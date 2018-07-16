package main

import (
	"fmt"
	"sync"

	"github.com/faiface/beep"
)

type StreamSequence struct {
	current    beep.StreamSeekCloser
	next       beep.StreamSeekCloser
	streamLock sync.Mutex
}

func NewStreamSequence(current beep.StreamSeekCloser) *StreamSequence {
	return &StreamSequence{
		current: current,
	}
}

func (self *StreamSequence) Stream(samples [][2]float64) (int, bool) {
	if stream, ok := self.stream(); ok {
		if n, ok := stream.Stream(samples); n == 0 && !ok {
			if self.SwapStreams() {
				return self.Stream(samples)
			} else {
				return 0, false
			}
		} else {
			return n, ok
		}
	} else {
		return 0, false
	}
}

func (self *StreamSequence) Err() error {
	if stream, ok := self.stream(); ok {
		return stream.Err()
	} else {
		return fmt.Errorf("no stream available")
	}
}

func (self *StreamSequence) Len() int {
	if stream, ok := self.stream(); ok {
		return stream.Len()
	} else {
		return 0
	}
}

func (self *StreamSequence) Position() int {
	if stream, ok := self.stream(); ok {
		return stream.Position()
	} else {
		return 0
	}
}

func (self *StreamSequence) Seek(p int) error {
	if stream, ok := self.stream(); ok {
		return stream.Seek(p)
	} else {
		return fmt.Errorf("no stream available")
	}
}

func (self *StreamSequence) Close() error {
	if stream, ok := self.stream(); ok {
		defer func() {
			if self.next != nil {
				self.next.Close()
				self.next = nil
			}
		}()

		return stream.Close()
	} else {
		return nil
	}
}

func (self *StreamSequence) SetNextStream(next beep.StreamSeekCloser) {
	self.next = next
}

func (self *StreamSequence) SwapStreams() bool {
	self.streamLock.Lock()
	defer self.streamLock.Unlock()

	self.current = self.next
	self.next = nil

	if self.current != nil {
		return true
	} else {
		return false
	}
}

func (self *StreamSequence) stream() (beep.StreamSeekCloser, bool) {
	self.streamLock.Lock()
	defer self.streamLock.Unlock()

	if self.current != nil {
		return self.current, true
	} else {
		return nil, false
	}
}
