package application

import (
	"fmt"
	"log"

	streamdomain "github.com/c1r5/open-streaming/src/modules/streaming/domain"
	enginedomain "github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"gorm.io/gorm"
)

type Service struct {
	engine enginedomain.ITorrentEngine
	db     *gorm.DB
}

func NewService(eng enginedomain.ITorrentEngine, db *gorm.DB) *Service {
	return &Service{engine: eng, db: db}
}

func (s *Service) GetStreamInfo(id uint) (*streamdomain.TorrentStreamInfo, error) {
	return s.byId(id)
}

func (s *Service) GetStreamFile(id uint) (*streamdomain.TorrentStreamFile, error) {
	record, err := s.byId(id)

	if err != nil {
		return nil, err
	}

	log.Printf("service: resolving stream id=%d hash=%s", id, record.Hash)
	file, err := s.engine.Resolve(record.Hash)

	if err != nil {
		return nil, err
	}

	log.Printf("service: resolved id=%d file=%s size=%d", id, file.Name, file.Size)
	return &streamdomain.TorrentStreamFile{
		Path:   file.Path,
		Reader: file.Reader,
	}, nil
}

func (s *Service) byId(id uint) (*streamdomain.TorrentStreamInfo, error) {
	var record streamdomain.TorrentStreamInfo
	if err := s.db.First(&record, id).Error; err != nil {
		return nil, fmt.Errorf("streaming -> torrent not found -> %w", err)
	}
	return &record, nil
}
