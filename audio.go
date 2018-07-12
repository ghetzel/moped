package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/moped/library"
)

type PlayableAudio struct {
	Entry    *library.Entry
	Stream   beep.StreamSeekCloser
	Format   beep.Format
	Control  *beep.Ctrl
	PlayChan chan struct{}
}

func (self *Moped) GetMetadata(reader io.Reader) (*library.Metadata, error) {
	buffer := bytes.NewBuffer(nil)

	if _, err := io.CopyN(buffer, reader, 65536); err != nil {
		return nil, err
	}

	args := []string{
		`-v`, `quiet`,
		`-show_format`,
		`-show_entries`, `stream=codec_name:format`,
		`-select_streams`, `a:0`,
		`-print_format`, `json`,
		`-`,
	}

	probe := exec.Command(`ffprobe`, args...)
	probe.Stdin = buffer
	probe.Env = []string{
		`AV_LOG_FORCE_NOCOLOR=1`,
	}

	if data, err := probe.Output(); err == nil {
		var metadata map[string]interface{}

		if err := json.Unmarshal(data, &metadata); err == nil {
			m := maputil.M(metadata)

			meta := &library.Metadata{
				Extra: metadata,
			}

			for _, field := range []string{
				`title`,
				`artist`,
				`album`,
			} {
				var value string

				if v := m.String(`format.tags.` + field); v != `` {
					value = v
				} else if v := m.String(`format.tags.` + strings.ToUpper(field)); v != `` {
					value = v
				}

				if value != `` {
					switch field {
					case `title`:
						meta.Title = value
					case `album`:
						meta.Album = value
					case `artist`:
						meta.Artist = value
					}
				}
			}

			for _, field := range []string{
				`tracknumber`,
				`track`,
				`discnumber`,
				`disc`,
			} {
				var value int64

				if v := m.Int(`format.tags.` + field); v > 0 {
					value = v
				} else if v := m.Int(`format.tags.` + strings.ToUpper(field)); v > 0 {
					value = v
				}

				if value > 0 {
					switch field {
					case `disc`, `discnumber`:
						meta.Disc = int(value)
					case `track`, `tracknumber`:
						meta.Track = int(value)
					}
				}
			}

			if seconds := m.Float(`format.duration`); seconds > 0 {
				meta.Duration = time.Duration(seconds * 1e9)
			} else if bitrate := m.Int(`format.bit_rate`); m.String(`format.format_name`) == `wav` && bitrate > 0 {
				size := int64(buffer.Len())

				if n, err := io.Copy(ioutil.Discard, reader); err == nil {
					size += n

					// milliseconds = ((bytes * 8) / bitrate) * 1000
					ms := (float64(size*8) / float64(bitrate)) * 1000
					meta.Duration = time.Duration(ms) * time.Millisecond
				}
			}

			log.Debug(meta.Duration.String())

			return meta, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
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
		audio.Stream = NewStreamSequence(stream)
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
		ss := self.playing.Stream
		return self.playing.Format.SampleRate.D(ss.Position())
	}

	return 0
}

func (self *Moped) Length() time.Duration {
	if self.playing != nil {
		ss := self.playing.Stream
		return self.playing.Format.SampleRate.D(ss.Len())
	}

	return 0
}

func (self *Moped) Seek(offset time.Duration) error {
	if self.playing != nil {
		ss := self.playing.Stream
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
		go func() {
			close(self.playing.PlayChan)
			self.playing = nil
			self.setState(StateStopped)
		}()

		return self.playing.Stream.Close()
	}

	return nil
}
