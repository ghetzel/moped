package main

import (
	"fmt"
	"path"

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

type Playlist struct {
	Items   library.EntryList
	current int
}

func (self *Playlist) Len() int {
	return len(self.Items)
}

func (self *Playlist) Index() int {
	return self.current
}

func (self *Playlist) Current() (*library.Entry, bool) {
	if self.current < len(self.Items) {
		return self.Items[self.current], true
	} else {
		return nil, false
	}
}

func (self *Playlist) Play() error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Pause() error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Resume() error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Stop() error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Next() error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Previous() error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Jump(index int) error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Seek(seconds float64) error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Remove(start int, end int) error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Move(start int, end int, to int) error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Info() []itemInfo {
	items := make([]itemInfo, len(self.Items))

	for i, entry := range self.Items {
		items[i] = itemInfo{
			Entry: entry,
			index: i,
		}
	}

	return items
}

func (self *Playlist) Insert(uri string, position int) error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Append(uri string) error {
	return self.Insert(uri, -1)
}

func (self *Playlist) Swap(i int, j int) error {
	return fmt.Errorf("Not Implemented")
}

func (self *Playlist) Shuffle() error {
	return fmt.Errorf("Not Implemented")
}
