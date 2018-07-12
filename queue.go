package main

import (
	"fmt"
	"path"

	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/moped/library"
)

type itemInfo struct {
	*library.Entry
	index int
}

func (self *itemInfo) String() string {
	var out string

	if v := self.Path; v != `` {
		out += fmt.Sprintf("file: %v\n", v)
	}

	if v := self.M().String(`title`); v != `` {
		out += fmt.Sprintf("Title: %v\n", v)
	} else {
		out += fmt.Sprintf("Title: %v\n", path.Base(self.Path))
	}

	out += fmt.Sprintf("Time: %d\n", 0)
	out += fmt.Sprintf("duration: %f\n", 0.0)

	out += fmt.Sprintf("Pos: %d\n", self.index)
	out += fmt.Sprintf("Id: %d\n", self.index)

	return out
}

type Queue struct {
	Items       library.EntryList
	current     int
	application *Moped
}

func NewQueue(app *Moped) *Queue {
	return &Queue{
		Items:       make(library.EntryList, 0),
		application: app,
	}
}

func (self *Queue) Len() int {
	return len(self.Items)
}

func (self *Queue) Index() int {
	return self.current
}

func (self *Queue) Current() (*library.Entry, bool) {
	if self.current < len(self.Items) {
		return self.Items[self.current], true
	} else {
		return nil, false
	}
}

func (self *Queue) Play() error {
	if self.current < len(self.Items) {
		entry := self.Items[self.current]
		return self.application.Play(entry)
	} else {
		return fmt.Errorf("Queue item %d does not exist", self.current)
	}
}

func (self *Queue) Next() error {
	started := self.current

	if err := self.JumpAndPlay(self.current + 1); err == nil {
		return nil
	} else if self.current > started {
		return self.Next()
	} else {
		return err
	}
}

func (self *Queue) Previous() error {
	return self.JumpAndPlay(self.current - 1)
}

func (self *Queue) HasNext() bool {
	if (self.current + 1) < len(self.Items) {
		return true
	}

	return false
}

func (self *Queue) Peek() (*library.Entry, bool) {
	if (self.current + 1) < len(self.Items) {
		return self.Items[self.current+1], true
	}

	return nil, false
}

func (self *Queue) JumpAndPlay(i int) error {
	if i < len(self.Items) {
		if err := self.Jump(i); err == nil {
			err := self.Play()

			if err != nil {
				log.Error(err)
			}

			return err
		} else {
			return err
		}
	} else {
		log.Warningf("Jump index %d out of bounds, stopping", i)
		return self.application.Stop()
	}
}

func (self *Queue) Jump(index int) error {
	if index < 0 {
		self.current = 0
	} else if index >= len(self.Items) {
		return fmt.Errorf("Cannot jump beyond end of queue")
	} else {
		self.current = index
	}

	return nil
}

func (self *Queue) Remove(start int, end int) error {
	return fmt.Errorf("Not Implemented")
}

func (self *Queue) Move(start int, end int, to int) error {
	return fmt.Errorf("Not Implemented")
}

func (self *Queue) Info() []itemInfo {
	items := make([]itemInfo, len(self.Items))

	for i, entry := range self.Items {
		items[i] = itemInfo{
			Entry: entry,
			index: i,
		}
	}

	return items
}

func (self *Queue) Insert(uri string, position int) error {
	if entries, err := self.application.Browse(uri); err == nil {
		var pre library.EntryList
		var post library.EntryList

		if position >= 0 && position < len(self.Items) {
			pre = self.Items[0:position]
			copy(post, self.Items[position:])
			self.Items = pre
		}

		for _, entry := range entries {
			if entry.IsContent() {
				self.Items = append(self.Items, entry)
			}
		}

		if len(post) > 0 {
			self.Items = append(self.Items, post...)
		}

		return nil
	} else {
		return err
	}
}

func (self *Queue) Append(uri string) error {
	return self.Insert(uri, -1)
}

func (self *Queue) Swap(i int, j int) error {
	return fmt.Errorf("Not Implemented")
}

func (self *Queue) Shuffle() error {
	return fmt.Errorf("Not Implemented")
}
