package moped

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
	"time"

	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/moped/library"
)

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
