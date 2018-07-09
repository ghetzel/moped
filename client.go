package main

import (
	"bufio"
	"io"
	"net"
	"regexp"
	"strings"

	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/kballard/go-shellquote"
)

var rxWhitespace = regexp.MustCompile(`\s+`)

type Client struct {
	id   string
	app  *Moped
	conn net.Conn
}

func NewClient(app *Moped, conn net.Conn) *Client {
	return &Client{
		id:   stringutil.UUID().String(),
		app:  app,
		conn: conn,
	}
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
			}

			switch c {
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

	_, err := w.Write(out)
	return err
}
