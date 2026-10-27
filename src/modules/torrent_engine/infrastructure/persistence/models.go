package persistence

import (
	"context"

	"gorm.io/gorm"
)

type TorrentModel struct {
	gorm.Model
	Hash string `gorm:"column:hash;uniqueIndex"`
	Name string `gorm:"column:name"`
	Size int64  `gorm:"column:size"`
}

func (TorrentModel) TableName() string { return "torrents" }

type MovieModel struct {
	gorm.Model
	TorrentID uint         `gorm:"column:torrent_id;uniqueIndex"`
	Torrent   TorrentModel `gorm:"foreignKey:TorrentID;references:ID;constraint:OnDelete:CASCADE"`
	FileIndex int          `gorm:"column:file_index"`
	Size      int64        `gorm:"column:size"`
	IMDB      *string      `gorm:"column:imdb;uniqueIndex"`
}

func (MovieModel) TableName() string { return "movies" }

type SeriesModel struct {
	gorm.Model
	TorrentID uint           `gorm:"column:torrent_id;uniqueIndex"`
	Torrent   TorrentModel   `gorm:"foreignKey:TorrentID;references:ID;constraint:OnDelete:CASCADE"`
	IMDB      *string        `gorm:"column:imdb;uniqueIndex"`
	Episodes  []EpisodeModel `gorm:"foreignKey:SeriesID;references:ID;constraint:OnDelete:CASCADE"`
}

func (SeriesModel) TableName() string { return "series" }

type EpisodeModel struct {
	gorm.Model
	SeriesID      uint        `gorm:"column:series_id;uniqueIndex:idx_series_file"`
	Series        SeriesModel `gorm:"foreignKey:SeriesID;references:ID;constraint:OnDelete:CASCADE"`
	FileIndex     int         `gorm:"column:file_index;uniqueIndex:idx_series_file"`
	Name          string      `gorm:"column:name"`
	Size          int64       `gorm:"column:size"`
	SubtitleIndex *int        `gorm:"column:subtitle_index"`
	SubtitleExt   string      `gorm:"column:subtitle_ext"`
}

func (EpisodeModel) TableName() string { return "episodes" }

func (m *TorrentModel) Create(db *gorm.DB) error {
	return gorm.G[TorrentModel](db).Create(context.Background(), m)
}

func (m *TorrentModel) Delete(db *gorm.DB) error {
	var current TorrentModel
	if err := db.Where("hash = ?", m.Hash).First(&current).Error; err != nil {
		return err
	}

	var series []SeriesModel
	if err := db.Where("torrent_id = ?", current.ID).Find(&series).Error; err != nil {
		return err
	}
	for _, item := range series {
		if err := db.Where("series_id = ?", item.ID).Delete(&EpisodeModel{}).Error; err != nil {
			return err
		}
	}
	if err := db.Where("torrent_id = ?", current.ID).Delete(&SeriesModel{}).Error; err != nil {
		return err
	}
	if err := db.Where("torrent_id = ?", current.ID).Delete(&MovieModel{}).Error; err != nil {
		return err
	}
	return db.Delete(&current).Error
}
