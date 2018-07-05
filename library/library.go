package library

type Library interface {
	Ping() error
	Browse(string) (EntryList, error)
	Get(string) (*Entry, error)
}
