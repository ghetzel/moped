package main

import (
	"fmt"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/moped/library"
)

type PlayableAudio struct {
	Entry    *library.Entry
	Stream   beep.StreamSeekCloser
	Format   beep.Format
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
		return fmt.Errorf("%v is already playing", self.playing.Entry.Path)
	} else {
		if entry.Type != library.AudioEntry {
			return fmt.Errorf("Can only play audio entries")
		}

		audio := &PlayableAudio{
			Entry: entry,
		}

		var stream beep.StreamSeekCloser
		var format beep.Format
		var err error

		switch mimetype := entry.MimeType(); mimetype {
		case `audio/mpeg`:
			stream, format, err = mp3.Decode(entry)
		case `audio/flac`:
			stream, format, err = flac.Decode(entry)
		case `audio/x-wav`:
			stream, format, err = wav.Decode(entry)
		default:
			return fmt.Errorf("Unsupported audio format %v", mimetype)
		}

		if err == nil {
			audio.Stream = stream
			audio.Format = format
			audio.PlayChan = make(chan struct{})

			if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err == nil {
				log.Debugf("Playing audio at %vHz", format.SampleRate.N(time.Second))
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
}

func (self *Moped) playAudio() {
	if self.playing != nil {
		speaker.Play(beep.Seq(self.playing.Stream, beep.Callback(func() {
			close(self.playing.PlayChan)
		})))

		<-self.playing.PlayChan
		self.playing = nil

		log.Debugf("Playback stopped")
	}
}

func (self *Moped) Stop() error {
	if self.playing != nil {
		defer func() {
			self.playing = nil
		}()

		return self.playing.Stream.Close()
	}

	return nil
}
