package main

import (
	"fmt"

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
	if v := self.Get(tag).String(); v != `` {
		return fmt.Sprintf("%v: %v\n", tag, v)
	} else {
		return ``
	}
}

func (self *Moped) cmdDbBrowse(c *cmd) *reply {
	switch c.Command {
	case `lsinfo`:
		if uri := c.Arg(0).String(); uri != `` {
			if entries, err := self.Browse(uri); err == nil {
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
		} else {
			return NewReply(c, fmt.Errorf("Must specify %q", `URI`))
		}

	case `listplaylistinfo`:
		return NewReply(c, nil)

	default:
		return NewReply(c, fmt.Errorf("Unsupported command %q", c.Command))
	}
}
