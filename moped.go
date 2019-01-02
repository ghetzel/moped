package moped

import (
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/moped/library"
	"github.com/ghetzel/moped/metadata"
)

var once sync.Once

type Moped struct {
	libraries map[string]library.Library
	commands  map[string]cmdHandler
	clients   sync.Map
	startedAt time.Time
}

func NewMoped() *Moped {
	once.Do(func() {
		metadata.SetupMimeTypes()
	})

	moped := &Moped{
		libraries: make(map[string]library.Library),
	}

	moped.commands = map[string]cmdHandler{
		`close`:            moped.cmdConnection,
		`commands`:         moped.cmdReflectCommands,
		`currentsong`:      moped.cmdCurrentSong,
		`decoders`:         moped.cmdReflectDecoders,
		`find`:             moped.cmdDbBrowse,
		`idle`:             moped.cmdIdle,
		`noidle`:           moped.cmdNoIdle,
		`kill`:             moped.cmdConnection,
		`listplaylistinfo`: moped.cmdDbBrowse,
		`listplaylists`:    moped.cmdPlaylistQueries,
		`lsinfo`:           moped.cmdDbBrowse,
		`list`:             moped.cmdDbBrowse,
		`notcommands`:      moped.cmdReflectNotCommands,
		`outputs`:          moped.cmdAudio,
		`password`:         moped.cmdConnection,
		`ping`:             moped.cmdConnection,
		`playlist`:         moped.cmdPlaylistQueries,
		`playlistid`:       moped.cmdPlaylistQueries,
		`playlistinfo`:     moped.cmdPlaylistQueries,
		`stats`:            moped.cmdStats,
		`status`:           moped.cmdStatus,
		`tagtypes`:         moped.cmdConnection,
		`urlhandlers`:      moped.cmdReflectUrlHandlers,
		// Not Implemented
		// NOTSUREIFWANT: https://www.musicpd.org/doc/protocol/mount.html
		// NOTSUREIFWANT: https://www.musicpd.org/doc/protocol/partition_commands.html
		// NOTSUREIFWANT: https://www.musicpd.org/doc/protocol/stickers.html
		// TODO: https://www.musicpd.org/doc/protocol/database.html
		// TODO: https://www.musicpd.org/doc/protocol/playlist_files.html
		// `add`:            moped.cmdPlaylistControl,
		// `addid`:          moped.cmdPlaylistControl,
		// `addtagid`:       moped.cmdPlaylistControl,
		// `clear`:          moped.cmdPlaylistControl,
		// `cleartagid`:     moped.cmdPlaylistControl,
		// `consume`:        moped.cmdToggles,
		// `delete`:         moped.cmdPlaylistControl,
		// `deleteid`:       moped.cmdPlaylistControl,
		// `disableoutput`:  moped.cmdAudio,
		// `enableoutput`:   moped.cmdAudio,
		// `move`:           moped.cmdPlaylistControl,
		// `moveid`:         moped.cmdPlaylistControl,
		// `next`:           moped.cmdPlayControl,
		// `outputset`:      moped.cmdAudio,
		// `pause`:          moped.cmdPlayControl,
		// `play`:           moped.cmdPlayControl,
		// `playid`:         moped.cmdPlayControl,
		// `playlistfind`:   moped.cmdPlaylistQueries,
		// `playlistsearch`: moped.cmdPlaylistQueries,
		// `plchanges`:      moped.cmdPlaylistQueries,
		// `plchangesposid`: moped.cmdPlaylistQueries,
		// `previous`:       moped.cmdPlayControl,
		// `prio`:           moped.cmdPlaylistControl,
		// `prioid`:         moped.cmdPlaylistControl,
		// `random`:         moped.cmdToggles,
		// `rangeid`:        moped.cmdPlaylistControl,
		// `repeat`:         moped.cmdToggles,
		// `seek`:           moped.cmdPlayControl,
		// `seekcur`:        moped.cmdPlayControl,
		// `seekid`:         moped.cmdPlayControl,
		// `shuffle`:        moped.cmdPlaylistControl,
		// `single`:         moped.cmdToggles,
		// `stop`:           moped.cmdPlayControl,
		// `swap`:           moped.cmdPlaylistControl,
		// `swapid`:         moped.cmdPlaylistControl,
		// `toggleoutput`:   moped.cmdAudio,
	}

	return moped
}

func (self *Moped) AddLibrary(name string, lib library.Library) error {
	if _, ok := self.libraries[name]; ok {
		return fmt.Errorf("library '%v' is already registered", name)
	} else if lib == nil {
		return fmt.Errorf("Cannot register nil library")
	}

	self.libraries[name] = lib
	log.Debugf("Registered %T library: %v", lib, name)
	return nil
}

func (self *Moped) Listen(network string, address string) error {
	if listener, err := net.Listen(network, address); err == nil {
		self.startedAt = time.Now()

		log.Infof("Listening on %v", listener.Addr())

		for {
			if conn, err := listener.Accept(); err == nil {
				go self.handleClient(conn)
			} else {
				log.Errorf("Client connection error: %v", err)
			}
		}
	} else {
		return err
	}
}

func (self *Moped) Ping() error {
	for name, lib := range self.libraries {
		if err := lib.Ping(); err != nil {
			return fmt.Errorf("library %v: %v", name, err)
		}
	}

	return nil
}

func (self *Moped) GetLibraryForPath(entryPath string) (string, string, library.Library, bool) {
	entryPath = strings.TrimPrefix(entryPath, `/`)

	if name, rest := stringutil.SplitPair(entryPath, `/`); name != `` {
		if lib, ok := self.libraries[name]; ok {
			return name, rest, lib, true
		} else {
			return name, rest, nil, false
		}
	}

	return ``, ``, nil, false
}

func (self *Moped) Browse(entryPath string) (library.EntryList, error) {
	if name, rest, lib, ok := self.GetLibraryForPath(entryPath); ok {
		if entries, err := lib.Browse(rest); err == nil {
			for _, entry := range entries {
				entry.SetParentPath(name)
			}

			return entries, nil
		} else {
			return nil, err
		}
	} else if name == `` {
		libraries := make(library.EntryList, 0)

		keys := maputil.StringKeys(self.libraries)
		sort.Strings(keys)

		for _, name := range keys {
			if _, ok := self.libraries[name]; ok {
				libraries = append(libraries, &library.Entry{
					Path: `/` + name,
					Type: library.FolderEntry,
				})
			}
		}

		return libraries, nil
	} else {
		return nil, fmt.Errorf("No such library '%v'", name)
	}
}

func (self *Moped) Get(entryPath string) (*library.Entry, error) {
	if name, rest, lib, ok := self.GetLibraryForPath(entryPath); ok {
		if entry, err := lib.Get(rest); err == nil {
			entry.SetParentPath(name)

			return entry, nil
		} else {
			return nil, err
		}
	} else if name == `` {
		return nil, fmt.Errorf("Must specify a path to retrieve")
	} else {
		return nil, fmt.Errorf("No such library '%v'", name)
	}
}

func (self *Moped) DropClient(id string) error {
	if clientI, ok := self.clients.Load(id); ok {
		defer func(cid string) {
			self.clients.Delete(cid)
			log.Debugf("Client %v removed", cid)
		}(id)

		return clientI.(*Client).Close()
	} else {
		return nil
	}
}

func (self *Moped) AddChangedSubsystem(subsystem string) {
	self.clients.Range(func(id interface{}, clientI interface{}) bool {
		clientI.(*Client).AddChangedSubsystem(subsystem)
		return true
	})
}

func (self *Moped) Stop() error {
	return nil
}

func (self *Moped) handleClient(conn net.Conn) {
	client := NewClient(self, conn)
	self.clients.Store(client.ID(), client)
	log.Debugf("Client %v connected via %v", client.ID(), conn.RemoteAddr())

	defer client.Run()
}

func (self *Moped) execute(w io.Writer, commands []*cmd) {
	for _, c := range commands {
		c.Reply = self.executeCommand(w, c)
		// log.Dump(c.Reply)
	}
}

func (self *Moped) executeCommand(w io.Writer, c *cmd) *reply {
	if handler, ok := self.commands[c.Command]; ok {
		return handler(c)
	} else {
		log.Errorf("Unsupported command '%v'", c.Command)
		return NewReply(c, fmt.Errorf("Unsupported command '%v'", c.Command))
	}
}
