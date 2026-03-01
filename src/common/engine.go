package common

import (
	"io"
)

type TorrentFile struct {
	Path     string
	Index    int
	Name string
	Size     int64
	Reader   io.ReadSeeker
}

type ITorrentEngine interface {
	Resolve(hash string) (*TorrentFile, error)
	PersistTorrentFile(hash string) error
}
