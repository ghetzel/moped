package library

type EntryType int

const (
	FileEntry EntryType = iota
	AudioEntry
	VideoEntry
	MetadataEntry
	FolderEntry
	PlaylistEntry
)

func (self EntryType) String() string {
	switch self {
	case AudioEntry:
		return `audio`
	case VideoEntry:
		return `video`
	case MetadataEntry:
		return `metadata`
	case FolderEntry:
		return `folder`
	case PlaylistEntry:
		return `playlist`
	default:
		return `file`
	}
}

func (self EntryType) MarshalJSON() ([]byte, error) {
	return []byte(self.String()), nil
}
