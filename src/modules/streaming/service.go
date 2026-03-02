package streaming

import (
	"fmt"

	"github.com/c1r5/open-streaming/src/common"
	"gorm.io/gorm"
)

type Service struct {
	engine common.ITorrentEngine
	db     *gorm.DB
}

func (s *Service) GetStreamInfo(id uint) (*common.TorrentStreamInfo, error) {
	return s.byId(id)
}

func (s *Service) GetStreamFile(id uint) (*common.TorrentStreamFile, error) {
	record, err := s.byId(id)

	if err != nil {
		return nil, err
	}

	file, err := s.engine.Resolve(record.Hash)

	if err != nil {
		return nil, err
	}

	return &common.TorrentStreamFile{
		Path:   file.Path,
		Reader: file.Reader,
	}, nil
}

func (s *Service) byId(id uint) (*common.TorrentStreamInfo, error) {
	var record common.TorrentStreamInfo
	if err := s.db.First(&record, id).Error; err != nil {
		return nil, fmt.Errorf("streaming -> torrent not found -> %w", err)
	}
	return &record, nil
}