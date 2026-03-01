package torrent

import (
	"context"

	"gorm.io/gorm"
)

type TorrentModel struct {
	gorm.Model
	FileIndex   int    `gorm:"column:file_index;index"`
	Hash        string `gorm:"column:hash;uniqueIndex"`
	TorrentName string `gorm:"column:torrent_name"`
	IMDB        string `gorm:"column:imdb;uniqueIndex"`
	OutputPath  string `gorm:"column:output"`
	Size        int64  `gorm:"column:size"`
}

func (m *TorrentModel) Create(db *gorm.DB) error {
	return gorm.G[TorrentModel](db).Create(context.Background(), m)
}

func (m *TorrentModel) Delete(db *gorm.DB) error {
	rows, err := gorm.G[TorrentModel](db).Where("hash = ?", m.Hash).Delete(context.Background())
	
	if err != nil {
		return err
	}
	
	if rows == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (m *TorrentModel) TableName() string {
	return "torrents"
}
