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

func (self *reply) IsError() bool {
	_, ok := self.Body.(error)
	return ok
}

func (self *reply) AddReply(reply *reply) {
	reply.Parent = self
	self.Subreplies = append(self.Subreplies, reply)
}

func (self *reply) String() string {
	out := make([]string, 0)

	if self.Body != nil {
		if typeutil.IsMap(self.Body) {
			maputil.Walk(self.Body, func(value interface{}, path []string, isLeaf bool) error {
				key := strings.Join(path, `.`)

				if typeutil.IsArray(value) {
					for _, v := range sliceutil.Sliceify(value) {
						out = append(out, key+`: `+strings.TrimSpace(fmt.Sprintf("%v", v)))
					}

					return maputil.SkipDescendants
				} else if isLeaf {
					out = append(out, key+`: `+strings.TrimSpace(fmt.Sprintf("%v", value)))
				}

				return nil
			})
		} else if s, err := stringutil.ToString(self.Body); err == nil && s != `` {
			out = []string{s}
		}
	}

	if self.Command != nil {
		if self.IsError() {
			return fmt.Sprintf("ACK [5@1] {%s} %v\n", self.Command.Command, self.Body)
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
