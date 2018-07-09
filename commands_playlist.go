package main

import (
	"fmt"

	"github.com/ghetzel/go-stockutil/stringutil"
)

func (self *Moped) cmdPlaylistQueries(c *cmd) *reply {
	switch c.Command {
	case `playlist`, `playlistinfo`:
		return NewReply(c, self.queue.Info())

	case `playlistid`:
		items := self.queue.Info()

		if v := c.Arg(0); !v.IsNil() {
			if id := int(v.Int()); id < len(items) {
				return NewReply(c, items[id])
			} else {
				return NewReply(c, fmt.Errorf("No such song"))
			}
		} else {
			return NewReply(c, items)
		}

	case `listplaylists`:
		return NewReply(c, nil)
	default:
		return NewReply(c, fmt.Errorf("Unsupported command %q", c.Command))
	}
}

func (self *Moped) cmdPlaylistControl(c *cmd) *reply {
	var err error
	switch c.Command {
	case `add`:
		err = self.queue.Append(c.Arg(0).String())

	case `addid`:
		if len(c.Arguments) < 1 {
			return NewReply(c, fmt.Errorf("Must specify %q", `URI`))
		}

		position := -1

		if p := c.Arg(1); !p.IsNil() {
			position = int(p.Int())
		}

		err = self.queue.Insert(c.Arg(0).String(), position)

	case `clear`:
		self.Stop()
		self.queue = NewQueue(self)

	case `delete`, `deleteid`:
		if start, end, rerr := getRangeFromCmd(c); err == nil {
			err = self.queue.Remove(start, end)
		} else {
			err = rerr
		}

	case `move`, `moveid`:
		if len(c.Arguments) < 2 {
			err = fmt.Errorf("Must specify %q/%q and %q", `FROM`, `START:END`, `TO`)
		} else if start, end, rerr := getRangeFromCmd(c); err == nil {
			err = self.queue.Move(start, end, int(c.Arg(1).Int()))
		} else {
			err = rerr
		}

	case `shuffle`:
		err = self.queue.Shuffle()

	case `swap`, `swapid`:
		if len(c.Arguments) < 2 {
			err = fmt.Errorf("Must specify %q and %q", `SONG1`, `SONG2`)
		} else {
			err = self.queue.Swap(
				int(c.Arg(0).Int()),
				int(c.Arg(1).Int()),
			)
		}
	}

	return NewReply(c, err)
}

func getRangeFromCmd(c *cmd) (int, int, error) {
	if len(c.Arguments) < 1 {
		return 0, 0, fmt.Errorf("Must specify %q or %q", `POS`, `START:END`)
	}

	var start int
	var end int

	a, b := stringutil.SplitPair(c.Arg(0).String(), `:`)

	if a != `` {
		if v, err := stringutil.ConvertToInteger(a); err == nil {
			start = int(v)
		} else {
			return 0, 0, err
		}
	}

	if b != `` {
		if v, err := stringutil.ConvertToInteger(b); err == nil {
			end = int(v)
		} else {
			return 0, 0, err
		}
	} else {
		end = -1
	}

	return start, end, nil
}
