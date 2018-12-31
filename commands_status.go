package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/ghetzel/go-stockutil/sliceutil"
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
		`repeat`:         b2i(false), // b2i(self.playmode.Repeat),
		`random`:         b2i(false), // b2i(self.playmode.Random),
		`single`:         b2i(false), // b2i(self.playmode.Single),
		`consume`:        b2i(false), // b2i(self.playmode.Consume),
		`playlist`:       1,
		`playlistlength`: 0, // self.queue.Len(),
		`mixrampdb`:      `0.000000`,
		`state`:          `stop`,
		`song`:           0,
		`songid`:         0,
	}

	// if next, ok := self.queue.Peek(); ok {
	// 	data[`nextsong`] = self.queue.Index() + 1
	// 	data[`nextsongid`] = next.ID()
	// }

	// switch self.state {
	// case StatePlaying, StatePaused:
	// 	position := self.Position()
	// 	length := self.Length()

	// 	data[`time`] = fmt.Sprintf(
	// 		"%d:%d",
	// 		int(position.Truncate(time.Second)/time.Second),
	// 		int(length.Truncate(time.Second)/time.Second),
	// 	)
	// 	data[`elapsed`] = float64(position / time.Second)
	// 	data[`duration`] = float64(length / time.Second)
	// }

	return NewReply(c, data)
}

func (self *Moped) cmdCurrentSong(c *cmd) *reply {
	return NotImplemented(c)

	// status := make(map[string]interface{})

	// if current, ok := self.queue.Current(); ok {
	// 	status[`file`] = current.Path

	// 	if v := current.Metadata.Year; v > 0 {
	// 		status[`Date`] = v
	// 	}

	// 	if v := current.Metadata.Track; v > 0 {
	// 		status[`Track`] = v
	// 	}

	// 	if v := current.Metadata.Album; v != `` {
	// 		status[`Album`] = v
	// 	}

	// 	if v := current.Metadata.Artist; v != `` {
	// 		status[`Artist`] = v
	// 	}

	// 	if v := current.Metadata.Title; v != `` {
	// 		status[`Title`] = v
	// 	}

	// 	if v := current.Metadata.Duration; v > 0 {
	// 		status[`Time`] = int(v.Round(time.Second) / time.Second)
	// 		status[`duration`] = float64(v.Round(time.Millisecond) / time.Millisecond)
	// 	}

	// 	status[`Pos`] = self.queue.Index()
	// 	status[`Id`] = current.ID()

	// 	return NewReply(c, status)
	// } else {
	// 	return NewReply(c, nil)
	// }
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

// Waits until there is a noteworthy change in one or more of MPD's subsystems. As soon as there is
// one, it lists all changed systems in a line in the format changed: SUBSYSTEM, where
// SUBSYSTEM is one of the following:
//
//   database:        the song database has been modified after update.
//   update:          a database update has started or finished. If the database was modified during
//                    the update, the database event is also emitted.
//   stored_playlist: a stored playlist has been modified, renamed, created or deleted
//   playlist:        the current playlist has been modified
//   player:          the player has been started, stopped or seeked
//   mixer:           the volume has been changed
//   output:          an audio output has been added, removed or modified (e.g. renamed, enabled or disabled)
//   options:         options like repeat, random, crossfade, replay gain
//   partition:       a partition was added, removed or changed
//   sticker:         the sticker database has been modified.
//   subscription:    a client has subscribed or unsubscribed to a channel
//   message:         a message was received on a channel this client is subscribed to;
//                    this event is only emitted when the queue is empty
//
func (self *Moped) cmdIdle(c *cmd) *reply {
	if client := c.Client; client != nil {
		defer func() {
			client.noidle = false
		}()

		for !client.noidle {
			if changes := client.RetrieveAndClearSubsystems(); len(changes) > 0 {
				return NewReply(c, `changed: `+strings.Join(changes, ` `))
			}

			if sliceutil.ContainsString(c.Arguments, `noidle`) || sliceutil.ContainsString(c.Arguments, `database`) {
				return NewReply(c, nil)
			}

			time.Sleep(125 * time.Millisecond)
		}
	}

	return NewReply(c, fmt.Errorf("client unavailable"))
}

func (self *Moped) cmdNoIdle(c *cmd) *reply {
	if client := c.Client; client != nil {
		client.noidle = true
		return NewReply(c, nil)
	}

	return NewReply(c, fmt.Errorf("client unavailable"))
}
