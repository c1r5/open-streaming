package application

import (
	"fmt"
	"log"

	streamdomain "github.com/c1r5/open-streaming/src/modules/streaming/domain"
	enginedomain "github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
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
	media, err := s.lookup(id)
	if err != nil {
		return nil, err
	}
	return &streamdomain.TorrentStreamInfo{
		Filename: media.filename,
		Hash:     media.hash,
		IMDB:     media.imdb,
		Size:     media.size,
	}, nil
}

func (s *Service) GetStreamFile(id uint) (*streamdomain.TorrentStreamFile, error) {
	media, err := s.lookup(id)
	if err != nil {
		return nil, err
	}

	log.Printf("service: resolving stream id=%d hash=%s episode=%t", id, media.hash, media.episode)
	var file *enginedomain.TorrentFile
	if media.episode {
		file, err = s.engine.ResolveFile(media.hash, media.fileIndex)
	} else {
		file, err = s.engine.Resolve(media.hash)
	}
	if err != nil {
		return nil, err
	}

	log.Printf("service: resolved id=%d file=%s size=%d", id, file.Name, file.Size)
	return &streamdomain.TorrentStreamFile{
		Path:   file.Path,
		Reader: file.Reader,
	}, nil
}

type streamMedia struct {
	filename  string
	hash      string
	imdb      string
	size      int64
	fileIndex int
	episode   bool
}

func (s *Service) lookup(id uint) (*streamMedia, error) {
	episode, real := enginedomain.ParseWatchID(id)
	if real == 0 {
		return nil, fmt.Errorf("streaming -> torrent not found")
	}
	if episode {
		var row persistence.EpisodeModel
		if err := s.db.Preload("Series.Torrent").First(&row, real).Error; err != nil {
			return nil, fmt.Errorf("streaming -> torrent not found -> %w", err)
		}
		imdb := ""
		if row.Series.IMDB != nil {
			imdb = *row.Series.IMDB
		}
		return &streamMedia{
			filename:  row.Name,
			hash:      row.Series.Torrent.Hash,
			imdb:      imdb,
			size:      row.Size,
			fileIndex: row.FileIndex,
			episode:   true,
		}, nil
	}

	var movie persistence.MovieModel
	if err := s.db.Preload("Torrent").First(&movie, real).Error; err != nil {
		return nil, fmt.Errorf("streaming -> torrent not found -> %w", err)
	}
	imdb := ""
	if movie.IMDB != nil {
		imdb = *movie.IMDB
	}
	return &streamMedia{
		filename: movie.Torrent.Name,
		hash:     movie.Torrent.Hash,
		imdb:     imdb,
		size:     movie.Size,
	}, nil
}
