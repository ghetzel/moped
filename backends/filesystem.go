package backends

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/pathutil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/moped/library"
	"github.com/ghetzel/moped/metadata"
	"github.com/mcuadros/go-defaults"
)

var LocalMetadataDetail = 1

type FilesystemConfig struct {
	Path string `json:"path"`
}

type FilesystemBackend struct {
	config *FilesystemConfig
}

func NewFilesystemBackend(config *FilesystemConfig) (*FilesystemBackend, error) {
	if config == nil {
		config = &FilesystemConfig{}
	}

	defaults.SetDefaults(config)

	if config.Path == `` {
		return nil, fmt.Errorf("Must specify a path for a filesystem library")
	}

	return &FilesystemBackend{
		config: config,
	}, nil
}

func (self *FilesystemBackend) Ping() error {
	if _, err := ioutil.ReadDir(self.config.Path); err != nil {
		return err
	}

	return nil
}

func (self *FilesystemBackend) Browse(relativePath string) (library.EntryList, error) {
	absPath := self.path(relativePath)

	if pathutil.FileExists(absPath) {
		if entry, err := self.Get(relativePath); err == nil {
			return library.EntryList{
				entry,
			}, nil
		} else {
			return nil, err
		}
	} else if infos, err := ioutil.ReadDir(absPath); err == nil {
		entries := make(library.EntryList, 0)

		for _, info := range infos {
			if entry, err := self.entryFromFileInfo(path.Join(absPath, info.Name()), info); err == nil {
				entries = append(entries, entry)
			} else {
				log.Warningf("Failed to read %v: %v", info.Name(), err)
				continue
			}
		}

		return entries, nil
	} else {
		return nil, err
	}
}

func (self *FilesystemBackend) Get(relativePath string) (*library.Entry, error) {
	absPath := self.path(relativePath)

	if info, err := os.Stat(absPath); err == nil {
		if entry, err := self.entryFromFileInfo(absPath, info); err == nil {
			return entry, nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (self *FilesystemBackend) path(relativePath string) string {
	relativePath = strings.TrimPrefix(relativePath, `/`)
	return path.Clean(path.Join(self.config.Path, relativePath))
}

func (self *FilesystemBackend) entryFromFileInfo(absPath string, info os.FileInfo) (*library.Entry, error) {
	relativePath := strings.TrimPrefix(absPath, self.config.Path)

	entry := &library.Entry{
		Path:     relativePath,
		Metadata: loadMetadata(absPath),
	}

	if info.IsDir() {
		entry.Type = library.FolderEntry
	} else {
		if mt, _ := stringutil.SplitPair(entry.MimeType(), `/`); mt != `application` {
			switch mt {
			case `audio`:
				entry.Type = library.AudioEntry
			case `video`:
				entry.Type = library.VideoEntry
			default:
				if path.Ext(info.Name()) == `.nfo` {
					entry.Type = library.MetadataEntry
				} else {
					entry.Type = library.FileEntry
				}
			}
		}

		entry.SetSource(library.NewLazyReader(func() (io.ReadCloser, error) {
			log.Debugf("File open: %v", absPath)
			return os.Open(absPath)
		}))
	}

	return entry, nil
}

func loadMetadata(filename string) library.Metadata {
	var meta library.Metadata
	data := make(map[string]interface{})

	for _, loader := range metadata.GetLoadersForFile(filename, LocalMetadataDetail) {
		if d, err := loader.LoadMetadata(filename); err == nil {
			data, _ = maputil.Merge(data, d)
		}
	}

	for key, value := range maputil.M(data).Map(`media`) {
		switch k := key.String(); k {
		case `title`:
			meta.Title = value.String()
		case `album`:
			meta.Album = value.String()
		case `artist`:
			meta.Artist = value.String()
		case `disc`:
			meta.Disc = int(value.Int())
		case `track`:
			meta.Track = int(value.Int())
		case `year`:
			meta.Year = int(value.Int())
		case `genre`:
			meta.Genre = value.String()
		case `duration`:
			if duration, ok := value.Value.(time.Duration); ok {
				meta.Duration = duration
			}
		default:
			if meta.Extra == nil {
				meta.Extra = make(map[string]interface{})
			}

			maputil.DeepSet(meta.Extra, strings.Split(k, `.`), value.Value)
		}
	}

	return meta
}
