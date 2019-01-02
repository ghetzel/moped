package moped

import (
	"fmt"
	"time"

	"github.com/ghetzel/moped/library"
)

type dbEntry struct {
	*library.Entry
}

func (self *dbEntry) String() string {
	var out string

	switch self.Type {
	case library.FolderEntry:
		out += fmt.Sprintf("directory: %v\n", self.FullPath())
	case library.PlaylistEntry:
		out += fmt.Sprintf("playlist: %v\n", self.FullPath())
	default:
		out += fmt.Sprintf("file: %v\n", self.FullPath())
	}

	out += self.stringEmIfYouGotEm(`Last-Modified`)
	out += self.stringEmIfYouGotEm(`Title`)
	out += self.stringEmIfYouGotEm(`Track`)
	out += self.stringEmIfYouGotEm(`Disc`)
	out += self.stringEmIfYouGotEm(`Artist`)
	out += self.stringEmIfYouGotEm(`Album`)

	return out
}

func (self *dbEntry) stringEmIfYouGotEm(tag string) string {
	if v := self.Get(tag); !v.IsNil() {
		if tm, ok := v.Value.(time.Time); ok {
			return fmt.Sprintf("%v: %v\n", tag, tm.Format(time.RFC3339))
		} else if vS := v.String(); vS != `` {
			return fmt.Sprintf("%v: %v\n", tag, v)
		}
	}

	return ``
}

func (self *Moped) entries(c *cmd, exprkey string, values ...string) *reply {
	var entries library.EntryList
	var err error

	switch exprkey {
	case `base`:
		if len(values) > 0 {
			entries, err = self.Browse(values[0])
		} else {
			entries, err = self.Browse(``)
		}
	default:
		err = fmt.Errorf("Unsupported expression: %v %v", exprkey, values)
	}

	if err == nil {
		results := make([]*dbEntry, 0)

		for _, entry := range entries {
			if !entry.IsHidden() && (entry.IsContainer() || entry.IsContent()) {
				results = append(results, &dbEntry{
					Entry: entry,
				})
			}
		}

		return NewReply(c, results)
	} else {
		return NewReply(c, err)
	}
}

func (self *Moped) cmdDbBrowse(c *cmd) *reply {
	switch c.Command {
	case `lsinfo`:
		return self.entries(c, `base`, c.Arg(0).String())

	case `list`:
		return NewReply(c, nil)

	case `listplaylistinfo`:
		return NewReply(c, nil)

	case `find`:
		return self.entries(c, c.Arguments[0], c.Arguments[1:]...)

	default:
		return NewReply(c, fmt.Errorf("Unsupported command %q", c.Command))
	}
}
