package main

import (
	"fmt"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/ghetzel/moped/library"
)

type PlayableAudio struct {
	Entry    *library.Entry
	Stream   beep.StreamSeekCloser
	Format   beep.Format
	Control  *beep.Ctrl
	PlayChan chan struct{}
}

func (self *Moped) Play(entry *library.Entry) error {
	return self.play(entry, false)
}

func (self *Moped) PlayAndWait(entry *library.Entry) error {
	return self.play(entry, true)
}

func (self *Moped) play(entry *library.Entry, block bool) error {
	if self.playing != nil {
		self.Stop()
	}

	if entry.Type != library.AudioEntry {
		return fmt.Errorf("Can only play audio entries")
	}

	audio := &PlayableAudio{
		Entry: entry,
	}

	if stream, format, err := ffmpegDecode(entry); err == nil {
		audio.Stream = stream
		audio.Format = format
		audio.PlayChan = make(chan struct{})

		if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err == nil {
			self.playing = audio

			if block {
				self.playAudio()
			} else {
				go self.playAudio()
			}

			return nil
		} else {
			return err
		}
	} else {
		return err
	}
}

func (self *Moped) playAudio() {
	if self.playing != nil {
		self.playing.Control = &beep.Ctrl{
			Streamer: beep.Seq(self.playing.Stream, beep.Callback(func() {
				self.Stop()
			})),
		}

		speaker.Play(self.playing.Control)
		self.setState(StatePlaying)

		if self.onAudioStart != nil {
			self.onAudioStart(self)
		}

		<-self.playing.PlayChan
		self.playing = nil

		if self.onAudioEnd != nil {
			self.onAudioEnd(self)
		}
	}
}

func (self *Moped) Pause() error {
	return self.setPaused(true)
}

func (self *Moped) Resume() error {
	return self.setPaused(false)
}

func (self *Moped) Position() time.Duration {
	if self.playing != nil {
		ss := self.playing.Stream.(beep.StreamSeeker)
		return self.playing.Format.SampleRate.D(ss.Position())
	}

	return 0
}

func (self *Moped) Length() time.Duration {
	if self.playing != nil {
		ss := self.playing.Stream.(beep.StreamSeeker)
		return self.playing.Format.SampleRate.D(ss.Len())
	}

	return 0
}

func (self *Moped) Seek(offset time.Duration) error {
	if self.playing != nil {
		ss := self.playing.Stream.(beep.StreamSeeker)
		sampleOffset := self.playing.Format.SampleRate.N(offset)
		length := ss.Len()

		if sampleOffset > length {
			sampleOffset = length
		}

		return ss.Seek(sampleOffset)
	} else {
		return fmt.Errorf("No stream is currently available")
	}
}

func (self *Moped) setPaused(on bool) error {
	if self.playing != nil {
		speaker.Lock()
		self.playing.Control.Paused = on

		if on {
			self.setState(StatePaused)
		} else {
			self.setState(StatePlaying)
		}

		speaker.Unlock()
		return nil
	} else {
		return fmt.Errorf("No stream is currently available")
	}
}

func (self *Moped) Stop() error {
	if self.playing != nil {
		close(self.playing.PlayChan)
		self.playing = nil
		self.setState(StateStopped)

		return self.playing.Stream.Close()
	}

	return nil
}
