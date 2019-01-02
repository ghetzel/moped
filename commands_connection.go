package moped

import "fmt"

func (self *Moped) cmdConnection(c *cmd) *reply {
	switch c.Command {
	case `close`:
		reply := NewReply(c, nil)
		reply.Directive = CloseConnection
		return reply

	case `kill`:
		return NewReply(c, fmt.Errorf("Killing the daemon is not supported"))

	case `password`:
		return NewReply(c, nil)

	case `ping`:
		return NewReply(c, nil)

	case `tagtypes`:
		if len(c.Arguments) == 0 {
			return NewReply(c, map[string]interface{}{
				`tagtypes`: []string{
					`Artist`,
					`Album`,
					`Title`,
					`Track`,
					`Disc`,
					`Name`,
					`Genre`,
					`Date`,
				},
			})
		} else {
			return NewReply(c, nil)
		}

	default:
		return NewReply(c, fmt.Errorf("Unsupported command %q", c.Command))
	}
}
