package main

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

type playmode struct {
	Consume   bool
	Random    bool
	Repeat    bool
	Single    bool
	Crossfade int
}

type Callback func(*Moped)
type StateCallback func(playstate, playstate, *Moped)

type playstate string

const (
	StateStopped playstate = `stop`
	StatePaused            = `pause`
	StatePlaying           = `play`
)

var MonitorInterval = 500 * time.Millisecond

type Moped struct {
	libraries          map[string]library.Library
	playing            *PlayableAudio
	queue              *Queue
	state              playstate
	playmode           playmode
	commands           map[string]cmdHandler
	clients            sync.Map
	startedAt          time.Time
	onAudioStart       Callback
	onStateChange      StateCallback
	aboutToEndDuration time.Duration
	onAudioAboutToEnd  Callback
	onAudioEnd         Callback
	aboutToEndFired    bool
}

func NewMoped() *Moped {
	once.Do(func() {
		metadata.SetupMimeTypes()
	})

	moped := &Moped{
		libraries: make(map[string]library.Library),
		state:     StateStopped,
		onAudioEnd: func(m *Moped) {
			if m.queue.HasNext() {
				log.Debugf("Track ended, playing next")
				m.queue.Next()
			} else {
				log.Debugf("Track ended, reached end of queue")
				m.Stop()
			}
		},
		onStateChange: func(is playstate, was playstate, m *Moped) {
			log.Debugf("State transition: %v -> %v", was, is)
		},
		onAudioStart: func(m *Moped) {
			log.Debugf("Playing audio at %vHz", m.playing.Format.SampleRate.N(time.Second))
		},
		aboutToEndDuration: (3 * time.Second),
		onAudioAboutToEnd: func(m *Moped) {
			log.Debugf("Audio about to end")
		},
	}

	moped.queue = NewQueue(moped)

	moped.commands = map[string]cmdHandler{
		`status`:       moped.cmdStatus,
		`stats`:        moped.cmdStats,
		`currentsong`:  moped.cmdCurrentSong,
		`commands`:     moped.cmdReflectCommands,
		`notcommands`:  moped.cmdReflectNotCommands,
		`urlhandlers`:  moped.cmdReflectUrlHandlers,
		`decoders`:     moped.cmdReflectDecoders,
		`consume`:      moped.cmdToggles,
		`random`:       moped.cmdToggles,
		`repeat`:       moped.cmdToggles,
		`single`:       moped.cmdToggles,
		`next`:         moped.cmdPlayControl,
		`previous`:     moped.cmdPlayControl,
		`pause`:        moped.cmdPlayControl,
		`play`:         moped.cmdPlayControl,
		`playid`:       moped.cmdPlayControl,
		`stop`:         moped.cmdPlayControl,
		`seek`:         moped.cmdPlayControl,
		`seekid`:       moped.cmdPlayControl,
		`seekcur`:      moped.cmdPlayControl,
		`playlist`:     moped.cmdPlaylistQueries,
		`playlistinfo`: moped.cmdPlaylistQueries,
		`playlistid`:   moped.cmdPlaylistQueries,
		// `playlistfind`:   moped.cmdPlaylistQueries,
		// `playlistsearch`: moped.cmdPlaylistQueries,
		// `plchanges`:      moped.cmdPlaylistQueries,
		// `plchangesposid`: moped.cmdPlaylistQueries,
		`listplaylists`:    moped.cmdPlaylistQueries,
		`listplaylistinfo`: moped.cmdDbBrowse,
		`lsinfo`:           moped.cmdDbBrowse,
		`add`:              moped.cmdPlaylistControl,
		`addid`:            moped.cmdPlaylistControl,
		`clear`:            moped.cmdPlaylistControl,
		`delete`:           moped.cmdPlaylistControl,
		`deleteid`:         moped.cmdPlaylistControl,
		`move`:             moped.cmdPlaylistControl,
		`moveid`:           moped.cmdPlaylistControl,
		`shuffle`:          moped.cmdPlaylistControl,
		`swap`:             moped.cmdPlaylistControl,
		`swapid`:           moped.cmdPlaylistControl,
		// `prio`:       moped.cmdPlaylistControl,
		// `prioid`:     moped.cmdPlaylistControl,
		// `rangeid`:    moped.cmdPlaylistControl,
		// `addtagid`:   moped.cmdPlaylistControl,
		// `cleartagid`: moped.cmdPlaylistControl,
		// TODO: https://www.musicpd.org/doc/protocol/playlist_files.html
		// TODO: https://www.musicpd.org/doc/protocol/database.html
		// NOTSUREIFWANT: https://www.musicpd.org/doc/protocol/mount.html
		// NOTSUREIFWANT: https://www.musicpd.org/doc/protocol/stickers.html
		// NOTSUREIFWANT: https://www.musicpd.org/doc/protocol/partition_commands.html
		`close`:    moped.cmdConnection,
		`kill`:     moped.cmdConnection,
		`password`: moped.cmdConnection,
		`ping`:     moped.cmdConnection,
		`tagtypes`: moped.cmdConnection,
		`outputs`:  moped.cmdAudio,
		// `disableoutput`: moped.cmdAudio,
		// `enableoutput`:  moped.cmdAudio,
		// `toggleoutput`:  moped.cmdAudio,
		// `outputset`:     moped.cmdAudio,
	}

	return moped
}

func (self *Moped) OnAudioStart(cb Callback) {
	self.onAudioStart = cb
}

func (self *Moped) OnStateChange(cb StateCallback) {
	self.onStateChange = cb
}

func (self *Moped) OnAudioAboutToEnd(timeFromEnd time.Duration, cb Callback) {
	if timeFromEnd < (2 * MonitorInterval) {
		timeFromEnd = (2 * MonitorInterval)
	}

	self.aboutToEndDuration = timeFromEnd
	self.onAudioAboutToEnd = cb
}

func (self *Moped) OnAudioEnd(cb Callback) {
	self.onAudioEnd = cb
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
		go self.monitor()

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

func (self *Moped) handleClient(conn net.Conn) {
	client := NewClient(self, conn)
	self.clients.Store(client.ID(), client)
	log.Debugf("Client %v connected via %v", client.ID(), conn.RemoteAddr())

	defer client.Run()
}

func (self *Moped) execute(w io.Writer, commands []*cmd) {
	for _, c := range commands {
		c.Reply = self.executeCommand(w, c)
	}
}

func (self *Moped) executeCommand(w io.Writer, c *cmd) *reply {
	if handler, ok := self.commands[c.Command]; ok {
		return handler(c)
	} else {
		return NewReply(c, fmt.Errorf("Unsupported command '%v'", c.Command))
	}
}

func (self *Moped) monitor() {
	for {
		switch self.state {
		case StatePlaying:
			if self.onAudioAboutToEnd != nil && !self.aboutToEndFired {
				if self.Position() >= (self.Length() - self.aboutToEndDuration) {
					self.onAudioAboutToEnd(self)
					self.aboutToEndFired = true
				}
			}
		}

		time.Sleep(MonitorInterval)
	}
}

func (self *Moped) setState(state playstate) {
	was := self.state
	self.state = state

	if self.onStateChange != nil {
		self.onStateChange(self.state, was, self)
	}
}
