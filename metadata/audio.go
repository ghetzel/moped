package metadata

import (
	"fmt"
	"os"

	"github.com/dhowden/tag"
	"github.com/ghetzel/go-stockutil/maputil"
)

type AudioLoader struct {
	Loader
	format   tag.Format
	filetype tag.FileType
	data     map[string]interface{}
}

func (self AudioLoader) CanHandle(name string) Loader {
	if file, err := os.Open(name); err == nil {
		defer file.Close()

		if format, filetype, err := tag.Identify(file); err == nil {
			return &AudioLoader{
				format:   format,
				filetype: filetype,
			}
		}
	}

	return nil
}

func (self AudioLoader) LoadMetadata(name string) (map[string]interface{}, error) {
	if file, err := os.Open(name); err == nil {
		defer file.Close()

		if metadata, err := tag.ReadFrom(file); err == nil {
			track, _ := metadata.Track()
			disc, _ := metadata.Disc()
			raw := maputil.M(metadata.Raw())

			self.data = map[string]interface{}{
				`media`: map[string]interface{}{
					`artist`:     metadata.Artist(),
					`album`:      metadata.Album(),
					`genre`:      metadata.Genre(),
					`title`:      metadata.Title(),
					`disc`:       disc,
					`track`:      track,
					`year`:       metadata.Year(),
					`comment`:    metadata.Comment(),
					`bitrate`:    raw.Int(`bitrate`),
					`channels`:   raw.Int(`channels`),
					`samplerate`: raw.Int(`samplerate`),
					// `duration`:   (metadata.Length() / time.Millisecond),
				},
			}

			return self.data, nil
		} else {
			return nil, fmt.Errorf("parse tags: %v", err)
		}
	} else {
		return nil, fmt.Errorf("read: %v", err)
	}
}
