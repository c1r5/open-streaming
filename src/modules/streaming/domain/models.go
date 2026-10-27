package domain

import "io"

type TorrentStreamFile struct {
	Path        string
	Reader      io.ReadSeeker
	ContentType string
}
