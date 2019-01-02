package moped

import (
	"sort"

	"github.com/ghetzel/go-stockutil/maputil"
)

func (self *Moped) cmdReflectCommands(c *cmd) *reply {
	keys := maputil.StringKeys(self.commands)
	sort.Strings(keys)

	return NewReply(c, map[string]interface{}{
		`commands`: keys,
	})
}

func (self *Moped) cmdReflectNotCommands(c *cmd) *reply {
	return NewReply(c, nil)
}

func (self *Moped) cmdReflectUrlHandlers(c *cmd) *reply {
	return NewReply(c, map[string]interface{}{
		`handlers`: []string{
			`http://`,
			`https://`,
		},
	})
}

func (self *Moped) cmdReflectDecoders(c *cmd) *reply {
	return NewReply(c, map[string]interface{}{
		`mime_type`: []string{
			`audio/flac`,
			`audio/mpeg`,
			`audio/x-flac`,
			`audio/x-mpeg`,
			`audio/x-wav`,
		},
		`suffix`: []string{
			`flac`,
			`mp3`,
			`wav`,
		},
		`plugin`: []string{
			`pcm`,
			`flac`,
		},
	})
}
