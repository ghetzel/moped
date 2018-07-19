package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/faiface/beep"

	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/pathutil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/go-stockutil/utils"
	"github.com/ghetzel/moped/library"
	"github.com/ghetzel/moped/metadata"
	"github.com/ghodss/yaml"
)

var once sync.Once
var CurrentQueueSyncPath = `~/.config/moped/state.yml`

type SaveState struct {
	URIs    []string `json:"uris"`
	Current int      `json:"current"`
}
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
	libraries            map[string]library.Library
	stream               *StreamSequence
	format               beep.Format
	queue                *Queue
	state                playstate
	playmode             playmode
	commands             map[string]cmdHandler
	clients              sync.Map
	startedAt            time.Time
	onAudioStart         Callback
	onStateChange        StateCallback
	aboutToEndDuration   time.Duration
	onAudioAboutToEnd    Callback
	onAudioEnd           Callback
	aboutToEndFired      bool
	autoAdvance          bool
	currentQueueSyncPath string
	playLock             sync.Mutex
	playChan             chan bool
}

func NewMoped() *Moped {
	once.Do(func() {
		metadata.SetupMimeTypes()
	})

	moped := &Moped{
		libraries:            make(map[string]library.Library),
		state:                StateStopped,
		autoAdvance:          true,
		currentQueueSyncPath: CurrentQueueSyncPath,
		playChan:             make(chan bool),
		format: beep.Format{
			SampleRate:  beep.SampleRate(EncodeSampleRate),
			NumChannels: 2,
			Precision:   2,
		},

		// handle audio start reporting
		onAudioStart: func(m *Moped) {
			log.Debugf("Playing audio")
		},

		// handle state change reporting
		onStateChange: func(is playstate, was playstate, m *Moped) {
			log.Debugf("State transition: %v -> %v (%d/%d)", was, is, m.stream.Position(), m.stream.Len())

			defer m.AddChangedSubsystem(`player`)
		},

		aboutToEndDuration: (3 * time.Second),

		// handle gapless audio playback
		// https://media.giphy.com/media/3oz8xtBx06mcZWoNJm/giphy.gif
		onAudioAboutToEnd: func(m *Moped) {
			log.Debugf("Audio about to end")
			defer m.AddChangedSubsystem(`player`)

			if m.autoAdvance {
				if next, ok := m.queue.Peek(); ok {
					if stream, _, err := ffmpegDecode(next); err == nil {
						m.stream.SetNextStream(stream)
						log.Debugf("Prepared next entry for gapless decoding")
					} else {
						log.Errorf("failed to prepare next entry: %v", err)
					}
				}
			}
		},

		// handle track auto-advance and queue completion
		onAudioEnd: func(m *Moped) {
			if m.autoAdvance && m.queue.HasNext() {
				log.Debugf("Track ended, playing next")
				m.queue.Next()
			} else {
				log.Debugf("Track ended, reached end of queue")
				m.Stop()
			}
		},
	}

	moped.queue = NewQueue(moped)

	moped.commands = map[string]cmdHandler{
		`status`:       moped.cmdStatus,
		`stats`:        moped.cmdStats,
		`idle`:         moped.cmdIdle,
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
	if err := self.setupAudioOutput(); err != nil {
		return fmt.Errorf("audio setup failed: %v", err)
	}

	if err := self.loadState(); err != nil {
		return fmt.Errorf("failed to load saved state: %v", err)
	}

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

func (self *Moped) AddChangedSubsystem(subsystem string) {
	self.clients.Range(func(id interface{}, clientI interface{}) bool {
		clientI.(*Client).AddChangedSubsystem(subsystem)
		return true
	})
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

func (self *Moped) saveState() error {
	if filename := self.currentQueueSyncPath; filename != `` {
		if filename, err := pathutil.ExpandUser(filename); err == nil {
			if file, err := os.Create(filename); err == nil {
				defer file.Close()

				if data, err := yaml.Marshal(&SaveState{
					URIs:    self.queue.CurrentURIs(),
					Current: self.queue.Index(),
				}); err == nil {
					_, err := file.Write(data)
					return err
				} else {
					return err
				}
			} else {
				return err
			}
		} else {
			return err
		}
	} else {
		return nil
	}
}

func (self *Moped) loadState() error {
	if filename := self.currentQueueSyncPath; filename != `` {
		if filename, err := pathutil.ExpandUser(filename); err == nil {
			if file, err := os.Open(filename); err == nil {
				var saved SaveState

				if data, err := ioutil.ReadAll(file); err == nil {
					if err := yaml.Unmarshal(data, &saved); err == nil {
						return self.applyState(&saved)
					} else {
						return err
					}
				} else {
					return err
				}
			} else if os.IsNotExist(err) {
				return nil
			} else {
				return err
			}
		} else {
			return err
		}
	} else {
		return nil
	}
}

func (self *Moped) applyState(state *SaveState) error {
	var multierr error

	// clear and load playlist, jump to correct entry
	if err := self.queue.Clear(); err == nil {
		if err := self.queue.Insert(-1, state.URIs...); err == nil {
			multierr = utils.AppendError(multierr, self.queue.Jump(state.Current))
		} else {
			multierr = utils.AppendError(multierr, err)
		}
	} else {
		multierr = utils.AppendError(multierr, err)
	}

	return multierr
}
