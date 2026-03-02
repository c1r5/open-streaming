package common

import (
	"io"

	"gorm.io/gorm"
)


type TorrentStreamInfo struct {
	gorm.Model
	Hash     string `gorm:"column:hash" json:"hash"`
	Filename string `gorm:"column:torrent_name" json:"filename"`
	IMDB     string `gorm:"column:imdb" json:"imdb"`
	Size     int64  `gorm:"column:size" json:"size"`
}

func (TorrentStreamInfo) TableName() string { return "torrents" }

type TorrentStreamFile struct {
	  Path   string
    Reader io.ReadSeeker
}


type IStreamingService interface {
	GetStreamInfo(id uint) (*TorrentStreamInfo, error)
	GetStreamFile(id uint) (*TorrentStreamFile, error)
}

type IStreamingController interface {
	StreamWatch() HandlerFunc
	GetStreamInfo() HandlerFunc
}
