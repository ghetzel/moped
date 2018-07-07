package library

import (
	"fmt"
	"io"
	"math/rand"
	"mime"
	"path"
	"sort"
	"strings"

	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/typeutil"
)

type EntryOrder string

const (
	OrderLinear             EntryOrder = ``
	OrderReverse                       = `reverse`
	OrderSortTitle                     = `sort-title`
	OrderRandom                        = `random`
	OrderRandomGroupArtists            = `random-by-artist`
	OrderRandomGroupAlbums             = `random-by-album`
	OrderRandomGroupYears              = `random-by-year`
)

type Entry struct {
	Path            string    `json:"path"`
	Type            EntryType `json:"type,omitempty"`
	Metadata        Metadata  `json:"metadata"`
	mimeOverride    string
	sortKeyOverride string
	source          io.ReadCloser
	parent          string
}

func (self *Entry) SetParentPath(path string) {
	self.parent = path
}

func (self *Entry) SetMimeType(mimetype string) {
	self.mimeOverride = mimetype
}

func (self *Entry) FullPath() string {
	return `/` + strings.TrimPrefix(path.Join(`/`, self.parent, self.Path), `/`)
}

func (self *Entry) String() string {
	out := fmt.Sprintf("[% 8v] %v", self.Type, self.FullPath())

	if self.Metadata.Title != `` {
		out += ":\nMetadata:\n"
		out += fmt.Sprintf("   title: %v\n", self.Metadata.Title)

		if artist := self.Metadata.Artist; artist != `` {
			out += fmt.Sprintf("  artist: %v\n", artist)
		}

		if album := self.Metadata.Album; album != `` {
			out += fmt.Sprintf("   album: %v\n", album)
		}

		if track := self.Metadata.Track; track > 0 {
			if disc := self.Metadata.Disc; disc > 0 {
				out += fmt.Sprintf("   track: %d/%02d\n", disc, track)
			} else {
				out += fmt.Sprintf("   track: %d\n", track)
			}
		}

		if self.Metadata.Year > 0 {
			out += fmt.Sprintf("    year: %d\n", self.Metadata.Year)
		}

		out += strings.Repeat(`-`, 80)
	}

	return out
}

func (self *Entry) IsHidden() bool {
	if strings.HasPrefix(path.Base(self.Path), `.`) {
		return true
	} else {
		return false
	}
}

func (self *Entry) IsContainer() bool {
	switch self.Type {
	case FolderEntry, PlaylistEntry:
		return true
	default:
		return false
	}
}

func (self *Entry) IsContent() bool {
	switch self.Type {
	case AudioEntry, VideoEntry:
		return true
	default:
		return false
	}
}

func (self *Entry) Name() string {
	if v := self.Get(`Title`); !v.IsNil() {
		return v.String()
	} else {
		return path.Base(self.Path)
	}
}

func (self *Entry) Get(field string) typeutil.Variant {
	switch f := strings.ToLower(field); f {
	case `filename`, `path`:
		return typeutil.V(self.FullPath())

	case `name`:
		return typeutil.V(self.Name())

	case `track`, `disc`, `year`:
		if v := self.M().Get(field); v.Int() > 0 {
			return v
		} else if v := self.M().Get(strings.ToLower(field)); v.Int() > 0 {
			return v
		} else {
			return typeutil.V(nil)
		}

	default:
		if v := self.M().Get(field); !v.IsNil() {
			return v
		} else if v := self.M().Get(strings.ToLower(field)); !v.IsNil() {
			return v
		} else {
			return typeutil.V(nil)
		}
	}
}

func (self *Entry) MimeType() string {
	if self.mimeOverride != `` {
		return self.mimeOverride
	} else if t := mime.TypeByExtension(path.Ext(self.Path)); t != `` {
		return t
	} else {
		return `application/octet-stream`
	}
}

func (self *Entry) M() *maputil.Map {
	return maputil.M(self.Metadata)
}

func (self *Entry) SetSource(rc io.ReadCloser) {
	self.source = rc
}

func (self *Entry) Read(b []byte) (int, error) {
	if self.source == nil {
		return 0, fmt.Errorf("Entry datasource not set")
	}

	return self.source.Read(b)
}

func (self *Entry) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := self.source.(io.Seeker); ok {
		return seeker.Seek(offset, whence)
	} else {
		return 0, fmt.Errorf("Underlying data source is not seekable")
	}
}

func (self *Entry) Close() error {
	if self.source != nil {
		return self.source.Close()
	}

	return nil
}

func (self *Entry) sortkey() string {
	if self.sortKeyOverride != `` {
		return self.sortKeyOverride
	}

	key := self.Path

	switch self.Type {
	case FolderEntry:
		key = `0:` + key
	case AudioEntry, VideoEntry, MetadataEntry:
		key = `1:` + key
	default:
		key = `2:` + key
	}

	return key
}

type EntryList []*Entry

func (self EntryList) Len() int {
	return len(self)
}

func (self EntryList) Less(i, j int) bool {
	eI := self[i]
	eJ := self[j]

	return (eI.sortkey() < eJ.sortkey())
}

func (self EntryList) Swap(i, j int) {
	eI := self[i]
	self[i] = self[j]
	self[j] = eI
}

// Reorder according to the specified ordering
func (self EntryList) Reorder(order EntryOrder) {
	length := len(self)
	randomGroups := make(map[string]int)

	for i, entry := range self {
		var groupKey string

		switch order {
		case OrderReverse:
			entry.sortKeyOverride = fmt.Sprintf("%d", length-i)

		case OrderRandom:
			entry.sortKeyOverride = fmt.Sprintf("%d", rand.Int())

		case OrderRandomGroupArtists:
			groupKey = typeutil.V(entry.Metadata.Artist).String()

		case OrderRandomGroupAlbums:
			groupKey = typeutil.V(entry.Metadata.Album).String()

		case OrderRandomGroupYears:
			groupKey = typeutil.V(entry.Metadata.Year).String()

		case OrderSortTitle:
			entry.sortKeyOverride = fmt.Sprintf("%v", entry.Metadata.Title)

		}

		if groupKey != `` {
			// generate the random number for this group key
			if _, ok := randomGroups[groupKey]; !ok {
				randomGroups[groupKey] = rand.Int()
			}
		}
	}

	// for random orders that preserve certain groupings, we make a second pass to get those keys together
	if len(randomGroups) > 0 {
		for _, entry := range self {
			var groupKey string

			switch order {
			case OrderRandomGroupArtists:
				groupKey = typeutil.V(entry.Metadata.Artist).String()

			case OrderRandomGroupAlbums:
				groupKey = typeutil.V(entry.Metadata.Album).String()

			case OrderRandomGroupYears:
				groupKey = typeutil.V(entry.Metadata.Year).String()
			}

			entry.sortKeyOverride = fmt.Sprintf("%d:%d", randomGroups[groupKey], rand.Int())
		}
	}

	// re-sort based on the new sort keys
	sort.Sort(self)
}
