package main

import (
	"fmt"
	"strings"

	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/sliceutil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/go-stockutil/typeutil"
)

type cmd struct {
	Command   string
	Arguments []string
	Reply     *reply
	Client    *Client
}

func (self *cmd) Arg(i int) typeutil.Variant {
	if i < len(self.Arguments) {
		return typeutil.V(self.Arguments[i])
	} else {
		return typeutil.V(nil)
	}
}

type ConnectionDirective int

const (
	NoOp ConnectionDirective = iota
	CloseConnection
)

type reply struct {
	Command    *cmd
	Body       interface{}
	Subreplies []*reply
	Parent     *reply
	Directive  ConnectionDirective
	NoTrailer  bool
}

func NewReply(cmd *cmd, body interface{}) *reply {
	return &reply{
		Command: cmd,
		Body:    body,
	}
}

func NotImplemented(cmd *cmd) *reply {
	return NewReply(cmd, fmt.Errorf("Command %q not implemented", cmd.Command))
}

func (self *reply) IsError() bool {
	_, ok := self.Body.(error)
	return ok
}

func (self *reply) AddReply(reply *reply) {
	reply.Parent = self
	self.Subreplies = append(self.Subreplies, reply)
}

func (self *reply) stringify(in interface{}) string {
	out := make([]string, 0)

	if in != nil {
		if typeutil.IsMap(in) {
			maputil.Walk(in, func(value interface{}, path []string, isLeaf bool) error {
				key := strings.Join(path, `.`)

				if typeutil.IsArray(value) {
					for _, v := range sliceutil.Sliceify(value) {
						out = append(out, fmt.Sprintf(
							"%s: %s",
							key,
							strings.TrimSpace(self.stringify(v)),
						))
					}

					return maputil.SkipDescendants
				} else if isLeaf {
					out = append(out, fmt.Sprintf(
						"%s: %s",
						key,
						strings.TrimSpace(self.stringify(value)),
					))
				}

				return nil
			})
		} else if typeutil.IsArray(in) {
			for _, v := range sliceutil.Sliceify(in) {
				vStr := strings.TrimSpace(self.stringify(v))
				out = append(out, vStr)
			}

		} else if s, err := stringutil.ToString(in); err == nil {
			if s != `` {
				out = []string{s}
			}
		}
	}

	if self.Command != nil {
		if self.IsError() {
			return fmt.Sprintf("ACK [5@1] {%s} %v\n", self.Command.Command, in)
		} else {
			if len(self.Subreplies) > 0 {
				for _, subreply := range self.Subreplies {
					if o := subreply.String(); strings.TrimSpace(o) != `` {
						out = append(out, strings.Split(o, "\n")...)
					}

					if self.Command.Command == `command_list_ok_begin` {
						out = append(out, "list_OK")
					}
				}
			}
		}
	}

	lines := make([]string, 0)

	for _, line := range out {
		if strings.TrimSpace(line) != `` {
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

func (self *reply) String() string {
	return self.stringify(self.Body)
}
