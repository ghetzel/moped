package main

import (
	"bufio"
	"fmt"
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
	noidle            bool
}

func NewClient(app *Moped, conn net.Conn) *Client {
	return &Client{
		id:   stringutil.UUID().String(),
		app:  app,
		conn: conn,
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
	return self.conn.Close()
}

func (self *Client) Run() {
	defer self.app.DropClient(self.ID())

	scanner := bufio.NewScanner(self.conn)
	commands := make([]*cmd, 0)

	var listCmd *cmd
	var inList bool
	var inCmd bool

	banner := NewReply(nil, `OK MPD 0.20.0`)
	banner.NoTrailer = true

	if err := self.writeReply(self.conn, banner); err != nil {
		log.Error(err)
		return
	}

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
			case `noidle`:
				commands = append(commands, command)
				inCmd = false
			case `status`, `outputs`:
				break
			default:
				log.Debugf("[%v] CMD: %v", self.ID(), line)
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
			}

			if c != `command_list_end` {
				commands = append(commands, command)
			} else {
				inList = false
			}

			if !inList {
				if !inCmd {
					inCmd = true

					go func() {
						self.app.execute(self.conn, commands)

						defer func() {
							inCmd = false
						}()

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

							if err := self.writeReply(self.conn, listReply); err == nil {
								listCmd = nil
								commands = nil
							} else {
								return
							}
						} else {
							for _, c := range commands {
								switch c.Reply.Directive {
								case CloseConnection:
									log.Warningf("Client %v closed connection", self.conn.RemoteAddr())
									return
								}

								if err := self.writeReply(self.conn, c.Reply); err == nil {
									commands = nil
								} else {
									return
								}
							}
						}
					}()
				} else if err := self.writeReply(self.conn, NewReply(command, fmt.Errorf("Another command is running"))); err == nil {
					commands = nil
				} else {
					return
				}
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

	log.Dumpf("reply: %v", out)

	_, err := w.Write(out)
	return err
}
