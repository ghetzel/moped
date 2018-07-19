package main

import (
	"fmt"
	"sync"
)

type StreamSequence struct {
	Mute       bool
	current    *decode
	next       *decode
	streamLock sync.Mutex
	app        *Moped
}

func NewStreamSequence(app *Moped) *StreamSequence {
	return &StreamSequence{
		app: app,
	}
}

func (self *StreamSequence) Stream(samples [][2]float64) (int, bool) {
	if !self.Mute {
		if stream, ok := self.stream(); ok {
			if n, ok := stream.Stream(samples); n == 0 && !ok {
				if self.SwapStreams() {
					// if self.app.onAudioEnd != nil {
					// 	self.app.onAudioEnd(self.app)
					// }

					if n, ok := self.Stream(samples); ok {
						if self.app.onAudioStart != nil {
							self.app.onAudioStart(self.app)
						}

						return n, ok
					}
				}
			} else {
				return n, ok
			}
		}
	}

	return len(samples), true
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
		return stream.Close()
	} else {
		return nil
	}
}

func (self *StreamSequence) ReplaceStream(stream *decode) {
	// self.streamLock.Lock()
	// defer self.streamLock.Unlock()

	self.current = stream
	self.next = nil
	self.Mute = false

}

func (self *StreamSequence) SetNextStream(next *decode) {
	// self.streamLock.Lock()
	// defer self.streamLock.Unlock()
	self.next = next
}

func (self *StreamSequence) SwapStreams() bool {
	// self.streamLock.Lock()
	// defer self.streamLock.Unlock()

	self.current = self.next
	self.next = nil
	self.Mute = false

	if self.current != nil {
		return true
	} else {
		return false
	}
}

func (self *StreamSequence) stream() (*decode, bool) {
	// self.streamLock.Lock()
	// defer self.streamLock.Unlock()

	if self.current != nil {
		return self.current, true
	} else {
		return nil, false
	}
}
