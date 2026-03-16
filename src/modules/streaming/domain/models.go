package domain

import (
	"io"

	"gorm.io/gorm"
)

type TorrentStreamInfo struct {
	gorm.Model
	Filename string `gorm:"column:torrent_name"`
	Hash     string `gorm:"column:hash"`
	IMDB     string `gorm:"column:imdb"`
	Size     int64  `gorm:"column:size"`
}

func (TorrentStreamInfo) TableName() string { return "torrents" }

type TorrentStreamFile struct {
	Path   string
	Reader io.ReadSeeker
}
