package streaming

import (
	"fmt"

	"github.com/c1r5/open-streaming/src/common"
	"gorm.io/gorm"
)

type torrentRecord struct {
	gorm.Model
	Hash string `gorm:"column:hash"`
}

func (torrentRecord) TableName() string { return "torrents" }

type Service struct {
	engine common.ITorrentEngine
	db     *gorm.DB
}

func (s *Service) GetStream(id uint) (*common.TorrentFile, error) {
	var record torrentRecord
	if err := s.db.First(&record, id).Error; err != nil {
		return nil, fmt.Errorf("streaming: torrent not found: %w", err)
	}
	return s.engine.Resolve(record.Hash)
}
