package moped

import "fmt"

func (self *Moped) cmdAudio(c *cmd) *reply {
	switch c.Command {
	case `outputs`:
		return NewReply(c, map[string]interface{}{
			`outputid`:      0,
			`outputname`:    `PulseAudio`,
			`outputenabled`: 1,
		})

	default:
		return NewReply(c, fmt.Errorf("Unsupported command %q", c.Command))
	}
}
