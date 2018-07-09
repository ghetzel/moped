package main

import (
	"fmt"
	"time"
)

type cmdHandler func(*cmd) *reply

// Reports the current status of the player and the volume level.
// - volume:         0-100 or -1 if the volume cannot be determined
// - repeat:         0 or 1
// - random:         0 or 1
// - single:         0, 1, or oneshot
// - consume:        0 or 1
// - playlist:       31-bit unsigned integer, the playlist version number
// - playlistlength: integer, the length of the playlist
// - state:          play, stop, or pause
// - song:           playlist song number of the current song stopped on or playing
// - songid:         playlist songid of the current song stopped on or playing
// - nextsong:       playlist song number of the next song to be played
// - nextsongid:     playlist songid of the next song to be played
// - time:           total time elapsed (of current playing/paused song)
// - elapsed:        Total time elapsed within the current song, but with higher resolution.
// - duration:       Duration of the current song in seconds.
// - bitrate:        instantaneous bitrate in kbps
// - xfade:          crossfade in seconds
// - mixrampdb:      mixramp threshold in dB
// - mixrampdelay:   mixrampdelay in seconds
// - audio:          The format emitted by the decoder plugin during playback, format: "samplerate:bits:channels".
//                   Check the user manual for a detailed explanation.
// - updating_db:    job id
// - error:          if there is an error, returns message here
//
func (self *Moped) cmdStatus(c *cmd) *reply {
	data := map[string]interface{}{
		`volume`:         -1,
		`repeat`:         b2i(self.playmode.Repeat),
		`random`:         b2i(self.playmode.Random),
		`single`:         b2i(self.playmode.Single),
		`consume`:        b2i(self.playmode.Consume),
		`playlist`:       1,
		`playlistlength`: self.queue.Len(),
		`mixrampdb`:      `0.000000`,
		`state`:          self.state,
		`song`:           self.queue.Index(),
		`songid`:         self.queue.Index(),
		`nextsong`:       4,
		`nextsongid`:     5,
	}

	switch self.state {
	case StatePlaying, StatePaused:
		position := self.Position()
		length := self.Length()

		data[`time`] = fmt.Sprintf(
			"%d:%d",
			int(position.Truncate(time.Second)/time.Second),
			int(length.Truncate(time.Second)/time.Second),
		)
		data[`elapsed`] = float64(position / time.Second)
		data[`duration`] = float64(length / time.Second)
	}

	return NewReply(c, data)
}

func (self *Moped) cmdCurrentSong(c *cmd) *reply {
	status := make(map[string]interface{})

	if current, ok := self.queue.Current(); ok {
		status[`file`] = current.Path

		if v := current.Metadata.Year; v > 0 {
			status[`Date`] = v
		}

		if v := current.Metadata.Track; v > 0 {
			status[`Track`] = v
		}

		if v := current.Metadata.Album; v != `` {
			status[`Album`] = v
		}

		if v := current.Metadata.Artist; v != `` {
			status[`Artist`] = v
		}

		if v := current.Metadata.Title; v != `` {
			status[`Title`] = v
		}

		if v := current.Metadata.Duration; v > 0 {
			status[`Time`] = int(v.Round(time.Second) / time.Second)
			status[`duration`] = float64(v.Round(time.Millisecond) / time.Millisecond)
		}

		status[`Pos`] = self.queue.Index()
		status[`Id`] = self.queue.Index()

		return NewReply(c, status)
	} else {
		return NewReply(c, nil)
	}
}

func b2i(in bool) int {
	if in {
		return 1
	} else {
		return 0
	}
}

// Displays statistics.
// - artists:     number of artists
// - albums:      number of albums
// - songs:       number of songs
// - uptime:      daemon uptime in seconds
// - db_playtime: sum of all song times in the db
// - db_update:   last db update in UNIX time
// - playtime:    time length of music played
//
func (self *Moped) cmdStats(c *cmd) *reply {
	return NewReply(c, map[string]interface{}{
		`artists`:     1,
		`albums`:      1,
		`songs`:       1,
		`uptime`:      int(time.Since(self.startedAt).Seconds()),
		`db_playtime`: 0,
		`db_update`:   time.Now().Unix(),
		`playtime`:    0,
	})
}
