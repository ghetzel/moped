package moped

import (
	"bufio"
	"io"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/kballard/go-shellquote"
)

var rxWhitespace = regexp.MustCompile(`\s+`)

type Client struct {
	id                string
	app               *Moped
	conn              net.Conn
	changedSubsystems sync.Map
	idle              bool
	running           bool
	cmdchan           chan cmdset
	replychan         chan *reply
}

func NewClient(app *Moped, conn net.Conn) *Client {
	return &Client{
		id:        stringutil.UUID().String(),
		app:       app,
		conn:      conn,
		running:   true,
		cmdchan:   make(chan cmdset),
		replychan: make(chan *reply),
	}
}

func (self *Client) AddChangedSubsystem(subsystem string) {
	self.changedSubsystems.Store(subsystem, true)
}

func (self *Client) RetrieveAndClearSubsystems() []string {
	changes := make([]string, 0)

	self.changedSubsystems.Range(func(key interface{}, _ interface{}) bool {
		if k := key.(string); k != `` {
			changes = append(changes, k)
		}

		return true
	})

	self.changedSubsystems = sync.Map{}

	return changes
}

func (self *Client) ID() string {
	return self.id
}

func (self *Client) Close() error {
	self.idle = false
	self.running = false

	close(self.replychan)
	return self.conn.Close()
}

func (self *Client) Run() {
	defer self.app.DropClient(self.ID())

	scanner := bufio.NewScanner(self.conn)
	commands := make(cmdset, 0)

	go self.runReplyLoop()

	var listCmd *cmd
	var inList bool

	banner := NewReply(nil, `OK MPD 0.20.0`)
	banner.NoTrailer = true

	self.pushReply(banner)

CommandLoop:
	for scanner.Scan() {
		if line := scanner.Text(); line != `` {
			c, args := self.parse(self.conn, line)

			if c == `` {
				log.Errorf("Malformed command, disconnecting")
				return
			}

			command := &cmd{
				Command:   c,
				Arguments: args,
				Client:    self,
			}

			switch c {
			case `status`, `outputs`:
				break
			default:
				// log.Debugf("[%v] CMD: %v", self.ID(), line)
			}

			switch c {
			case `command_list_begin`:
				listCmd = command
				inList = true
				continue CommandLoop
			case `command_list_ok_begin`:
				listCmd = command
				inList = true
				continue CommandLoop
			case `noidle`:
				self.idle = false
				continue CommandLoop
			}

			if c != `command_list_end` {
				commands = append(commands, command)
			} else {
				inList = false
			}

			if !inList {
				go self.execute(commands, listCmd)
				commands = nil
				listCmd = nil
			} else {
				log.Debugf("Command appended to list-in-progress")
			}
		}
	}
}

func (self *Client) parse(w io.Writer, line string) (string, []string) {
	if args, err := shellquote.Split(line); err == nil {
		cmd := args[0]
		args = args[1:]

		for i, arg := range args {
			args[i] = stringutil.Unwrap(arg, `"`, `"`)
		}

		return cmd, args
	} else {
		return ``, nil
	}
}

func (self *Client) writeReply(w io.Writer, reply *reply) error {
	body := strings.TrimSpace(reply.String())

	if !reply.NoTrailer && !reply.IsError() {
		if body != `` {
			body += "\n"
		}

		body += "OK"
	}

	out := []byte(body + "\n")

	// log.Dumpf("reply: %v", out)

	_, err := w.Write(out)
	return err
}

func (self *Client) execute(commands cmdset, listCmd *cmd) {
	self.app.execute(self.conn, commands)

	if listCmd != nil {
		listReply := NewReply(listCmd, ``)

		for _, c := range commands {
			switch c.Reply.Directive {
			case CloseConnection:
				log.Warningf("Client %v closed connection", self.conn.RemoteAddr())
				return
			}

			listReply.AddReply(c.Reply)
		}

		self.pushReply(listReply)
	} else {
		for _, c := range commands {
			switch c.Reply.Directive {
			case CloseConnection:
				log.Warningf("Client %v closed connection", self.conn.RemoteAddr())
				return
			}

			self.pushReply(c.Reply)
		}
	}
}

func (self *Client) pushReply(r *reply) {
	if self.running && r != nil {
		self.replychan <- r
	}
}

func (self *Client) runReplyLoop() {
	for r := range self.replychan {
		if err := self.writeReply(self.conn, r); err != nil {
			// log.Errorf("[%v] %v", self.ID(), err)
			self.Close()
			return
		}
	}
}
