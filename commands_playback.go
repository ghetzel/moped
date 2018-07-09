package main

import (
	"fmt"
	"time"

	"github.com/ghetzel/go-stockutil/typeutil"
)

func (self *Moped) cmdToggles(c *cmd) *reply {
	if len(c.Arguments) == 0 {
		state := typeutil.V(c.Arguments[0]).Bool()

		switch c.Command {
		case `consume`:
			self.playmode.Consume = state
		case `random`:
			self.playmode.Random = state
		case `repeat`:
			self.playmode.Repeat = state
		case `single`:
			self.playmode.Single = state
		case `crossfade`:
			self.playmode.Crossfade = int(typeutil.V(c.Arguments[0]).Int())
		default:
			return NewReply(c, fmt.Errorf("Unsupported state command %q", c.Command))
		}

		return NewReply(c, nil)
	} else {
		return NewReply(c, fmt.Errorf("wrong number of arguments for %q", c.Command))
	}
}

func (self *Moped) cmdPlayControl(c *cmd) *reply {
	var err error

	arg := c.Arg(0)

	switch c.Command {
	case `next`:
		err = self.queue.Next()

	case `previous`:
		err = self.queue.Previous()

	case `pause`:
		if arg.IsNil() || arg.Bool() {
			err = self.Pause()
		} else {
			err = self.Resume()
		}
	case `play`, `playid`:
		if arg.Value != nil {
			if err := self.queue.Jump(int(arg.Int())); err != nil {
				return NewReply(c, err)
			}
		}

		err = self.queue.Play()

	case `stop`:
		err = self.Stop()

	case `seek`, `seekid`:
		if len(c.Arguments) < 2 {
			return NewReply(c, fmt.Errorf("Must specify %q and %q", `POS`, `TIME`))
		}

		if id := int(arg.Int()); id > 0 {
			if err := self.queue.Jump(id); err != nil {
				return NewReply(c, err)
			}
		}

		fallthrough

	case `seekcur`:
		if len(c.Arguments) < 2 {
			return NewReply(c, fmt.Errorf("Must specify %q and %q", `POS`, `TIME`))
		}

		offset := time.Duration(c.Arg(1).Float()) * time.Second
		err = self.Seek(offset)

	default:
		return NewReply(c, fmt.Errorf("Unsupported command %q", c.Command))
	}

	return NewReply(c, err)
}
