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

func (s *Service) GetMovieFile(id uint) (*streamdomain.TorrentStreamFile, error) {
	var movie persistence.MovieModel
	if err := s.db.Preload("Torrent").First(&movie, id).Error; err != nil {
		return nil, fmt.Errorf("streaming -> movie not found -> %w", err)
	}
	log.Printf("service: resolving movie id=%d hash=%s", id, movie.Torrent.Hash)
	file, err := s.engine.Resolve(movie.Torrent.Hash)
	if err != nil {
		return nil, err
	}
	log.Printf("service: resolved movie id=%d file=%s size=%d", id, file.Name, file.Size)
	return &streamdomain.TorrentStreamFile{Path: file.Path, Reader: file.Reader}, nil
}

func (s *Service) GetEpisodeFile(seriesID, episodeID uint) (*streamdomain.TorrentStreamFile, error) {
	var episode persistence.EpisodeModel
	err := s.db.Preload("Series.Torrent").Where("series_id = ?", seriesID).First(&episode, episodeID).Error
	if err != nil {
		return nil, fmt.Errorf("streaming -> episode not found -> %w", err)
	}
	log.Printf("service: resolving episode series=%d id=%d hash=%s index=%d", seriesID, episodeID, episode.Series.Torrent.Hash, episode.FileIndex)
	file, err := s.engine.ResolveFile(episode.Series.Torrent.Hash, episode.FileIndex)
	if err != nil {
		return nil, err
	}
	log.Printf("service: resolved episode id=%d file=%s size=%d", episodeID, file.Name, file.Size)
	return &streamdomain.TorrentStreamFile{Path: file.Path, Reader: file.Reader}, nil
}
