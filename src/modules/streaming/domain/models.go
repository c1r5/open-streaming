package domain

import "io"

type TorrentStreamInfo struct {
	Filename string
	Hash     string
	IMDB     string
	Size     int64
}

type TorrentStreamFile struct {
	Path   string
	Reader io.ReadSeeker
}
