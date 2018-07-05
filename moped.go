package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/go-stockutil/typeutil"
	"github.com/ghetzel/moped/library"
	"github.com/ghetzel/moped/metadata"
)

var once sync.Once

type cmd struct {
	Command   string
	Arguments []string
	Reply     *reply
}

type reply struct {
	Command    *cmd
	Body       interface{}
	Subreplies []*reply
	Parent     *reply
}

func NewReply(cmd *cmd, body interface{}) *reply {
	return &reply{
		Command: cmd,
		Body:    body,
	}
}

func (self *reply) AddReply(reply *reply) {
	reply.Parent = self
	self.Subreplies = append(self.Subreplies, reply)
}

func (self *reply) String() string {
	var out string

	if self.Body != nil {
		if typeutil.IsMap(self.Body) {
			out = maputil.Join(self.Body, `: `, "\n")
		} else if s, err := stringutil.ToString(self.Body); err == nil {
			out = s
		}
	}

	if self.Command != nil {
		if err, ok := self.Body.(error); ok {
			return fmt.Sprintf("ACK [5@1] {%s} %v\n", self.Command.Command, err)
		} else {
			if len(self.Subreplies) > 0 {
				for _, subreply := range self.Subreplies {
					if o := subreply.String(); strings.TrimSpace(o) != `` {
						out += o + "\n"
					}

					if self.Command.Command == `command_list_ok_begin` {
						out += "list_OK\n"
					}
				}
			}
		}
	} else {
		return out + "\n"
	}

	if self.Parent == nil {
		out += "OK\n"
	}

	return out
}

type Moped struct {
	libraries map[string]library.Library
	playing   *PlayableAudio
	playlist  library.EntryList
	index     int
}

func NewMoped() *Moped {
	once.Do(func() {
		metadata.SetupMimeTypes()
	})

	return &Moped{
		libraries: make(map[string]library.Library),
	}
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

func (self *Moped) handleClient(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	commands := make([]*cmd, 0)

	var listCmd *cmd
	var inList bool

	if err := self.writeReply(conn, NewReply(nil, `OK MPD 0.20.0`)); err != nil {
		log.Error(err)
		return
	}

CommandLoop:
	for scanner.Scan() {
		if line := scanner.Text(); line != `` {
			c, args := self.parse(conn, line)
			command := &cmd{
				Command:   c,
				Arguments: args,
			}

			log.Debugf("CMD: %v", line)

			switch c {
			case `command_list_begin`:
				listCmd = command
				inList = true
				continue CommandLoop
			case `command_list_ok_begin`:
				listCmd = command
				inList = true
				continue CommandLoop
			}

			if c != `command_list_end` {
				commands = append(commands, command)
			} else {
				inList = false
			}

			if !inList {
				self.execute(conn, commands)

				if listCmd != nil {
					listReply := NewReply(listCmd, ``)

					for _, c := range commands {
						listReply.AddReply(c.Reply)
					}

					self.writeReply(conn, listReply)
					listCmd = nil
				} else {
					for _, c := range commands {
						self.writeReply(conn, c.Reply)
					}
				}
			}
		}
	}
}

func (self *Moped) parse(w io.Writer, line string) (string, []string) {
	args := regexp.MustCompile(`\s+`).Split(line, -1)
	return args[0], args[1:]
}

func (self *Moped) execute(w io.Writer, commands []*cmd) {
	for _, c := range commands {
		log.Debugf("EXEC: %v %v", c.Command, c.Arguments)
		c.Reply = self.executeCommand(w, c)
	}
}

func (self *Moped) executeCommand(w io.Writer, c *cmd) *reply {
	switch c.Command {
	case `currentsong`:
		status := make(map[string]interface{})

		if self.index < len(self.playlist) {
			current := self.playlist[self.index]
			status[`file`] = current.Path

			if v := current.Metadata.Year; v > 0 {
				status[`Date`] = v
			}

			if v := current.Metadata.Track; v > 0 {
				status[`Track`] = v
			}

			if v := current.Metadata.Album; v != `` {
				status[`Album`] = v
			}

			if v := current.Metadata.Artist; v != `` {
				status[`Artist`] = v
			}

			if v := current.Metadata.Title; v != `` {
				status[`Title`] = v
			}

			if v := current.Metadata.Duration; v > 0 {
				status[`Time`] = int(v.Round(time.Second) / time.Second)
				status[`duration`] = float64(v.Round(time.Millisecond) / time.Millisecond)
			}

			status[`Pos`] = self.index
			status[`Id`] = self.index

			return NewReply(c, status)
		} else {
			return NewReply(c, nil)
		}

	case `status`:
		return NewReply(c, map[string]interface{}{
			`volume`:         -1,
			`repeat`:         0,
			`random`:         0,
			`single`:         0,
			`consume`:        0,
			`playlist`:       2,
			`playlistlength`: len(self.playlist),
			`mixrampdb`:      0.000000,
			`state`:          `stop`,
			`song`:           3,
			`songid`:         4,
			`nextsong`:       4,
			`nextsongid`:     5,
		})
	}

	return NewReply(c, fmt.Errorf("Unsupported command '%v'", c.Command))
}

func (self *Moped) writeReply(w io.Writer, reply *reply) error {
	_, err := w.Write([]byte(reply.String()))
	return err
}
