package application

import (
	"errors"
	"fmt"
	"log"
	"path"
	"strings"

	catalogdomain "github.com/c1r5/open-streaming/src/modules/catalog/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/cache"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	"gorm.io/gorm"
)

type Service struct {
	searchCache   *cache.Cache[*catalogdomain.TorrentSearchResult]
	torrentEngine domain.ITorrentEngine
	torrentSearch catalogdomain.ITorrentSearch
	db            *gorm.DB
}

func (s *Service) AddTorrent(options *domain.AddTorrentOptions) (uint, error) {
	var (
		magnet *domain.MagnetModel
		imdb   *string
		err    error
	)

	if options.Magnet != "" {
		magnet, err = domain.Parse(options.Magnet)
		if err != nil {
			return 0, err
		}
	} else if options.Hash != "" {
		magnet = &domain.MagnetModel{Hash: options.Hash}
		searchResult, cacheErr := s.searchCache.Get(options.Hash)
		if cacheErr != nil {
			return 0, fmt.Errorf("failed to get torrent from cache: %w", cacheErr)
		}
		if searchResult.IMDB != "" {
			id := searchResult.IMDB
			imdb = &id
		}
	} else {
		return 0, fmt.Errorf("hash or magnet required")
	}

	videos, err := s.torrentEngine.ListVideos(magnet)
	if err != nil {
		return 0, fmt.Errorf("failed to resolve torrent: %w", err)
	}
	videos = domain.VideosForLibrary(videos)
	if len(videos) == 0 {
		return 0, fmt.Errorf("failed to resolve torrent: no video files")
	}
	domain.SortVideos(videos)

	if err := s.torrentEngine.PersistTorrentFile(magnet.Hash); err != nil {
		log.Printf("warn: could not persist .torrent for hash=%s: %v", magnet.Hash, err)
	}

	return s.saveLibrary(magnet.Hash, videos, imdb)
}

func (s *Service) saveLibrary(hash string, videos []domain.VideoMeta, imdb *string) (uint, error) {
	var total int64
	for _, video := range videos {
		total += video.Size
	}
	name := domain.PackTitle(videos[0].Name)

	var torrent persistence.TorrentModel
	err := s.db.Where("hash = ?", hash).First(&torrent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		torrent = persistence.TorrentModel{Hash: hash, Name: name, Size: total}
		if err := s.db.Create(&torrent).Error; err != nil {
			return 0, err
		}
	} else if err != nil {
		return 0, err
	} else if err := s.db.Model(&torrent).Updates(map[string]any{"name": name, "size": total}).Error; err != nil {
		return 0, err
	}

	if domain.IsSeries(videos) {
		if err := s.db.Where("torrent_id = ?", torrent.ID).Delete(&persistence.MovieModel{}).Error; err != nil {
			return 0, err
		}
		series, err := s.upsertSeries(torrent.ID, imdb)
		if err != nil {
			return 0, err
		}
		return s.upsertEpisodes(series.ID, videos)
	}

	if err := s.deleteSeries(torrent.ID); err != nil {
		return 0, err
	}
	return s.upsertMovie(torrent.ID, videos[0], imdb)
}

func (s *Service) upsertSeries(torrentID uint, imdb *string) (persistence.SeriesModel, error) {
	var series persistence.SeriesModel
	err := s.db.Where("torrent_id = ?", torrentID).First(&series).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		series = persistence.SeriesModel{TorrentID: torrentID, IMDB: imdb}
		err = s.db.Create(&series).Error
		return series, err
	}
	if err != nil {
		return series, err
	}
	if imdb != nil {
		err = s.db.Model(&series).Update("imdb", *imdb).Error
	}
	return series, err
}

func (s *Service) upsertEpisodes(seriesID uint, videos []domain.VideoMeta) (uint, error) {
	var existing []persistence.EpisodeModel
	if err := s.db.Where("series_id = ?", seriesID).Find(&existing).Error; err != nil {
		return 0, err
	}
	byIndex := make(map[int]persistence.EpisodeModel, len(existing))
	for _, episode := range existing {
		byIndex[episode.FileIndex] = episode
	}

	kept := map[uint]bool{}
	var firstID uint
	for _, video := range videos {
		base := path.Base(strings.ReplaceAll(video.Name, "\\", "/"))
		episode, ok := byIndex[video.Index]
		if !ok {
			episode = persistence.EpisodeModel{SeriesID: seriesID, FileIndex: video.Index, Name: base, Size: video.Size}
			if err := s.db.Create(&episode).Error; err != nil {
				return 0, err
			}
		} else if err := s.db.Model(&episode).Updates(map[string]any{"name": base, "size": video.Size}).Error; err != nil {
			return 0, err
		}
		kept[episode.ID] = true
		if firstID == 0 {
			firstID = episode.ID
		}
	}
	for _, episode := range existing {
		if kept[episode.ID] {
			continue
		}
		if err := s.db.Delete(&persistence.EpisodeModel{}, episode.ID).Error; err != nil {
			return 0, err
		}
	}
	return domain.EpisodeWatchID(firstID), nil
}

func (s *Service) upsertMovie(torrentID uint, video domain.VideoMeta, imdb *string) (uint, error) {
	var movie persistence.MovieModel
	err := s.db.Where("torrent_id = ?", torrentID).First(&movie).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		movie = persistence.MovieModel{TorrentID: torrentID, FileIndex: video.Index, Size: video.Size, IMDB: imdb}
		if err := s.db.Create(&movie).Error; err != nil {
			return 0, err
		}
		return domain.MovieWatchID(movie.ID), nil
	}
	if err != nil {
		return 0, err
	}
	updates := map[string]any{"file_index": video.Index, "size": video.Size}
	if imdb != nil {
		updates["imdb"] = *imdb
	}
	if err := s.db.Model(&movie).Updates(updates).Error; err != nil {
		return 0, err
	}
	return domain.MovieWatchID(movie.ID), nil
}

func (s *Service) deleteSeries(torrentID uint) error {
	var series persistence.SeriesModel
	err := s.db.Where("torrent_id = ?", torrentID).First(&series).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.db.Where("series_id = ?", series.ID).Delete(&persistence.EpisodeModel{}).Error; err != nil {
		return err
	}
	return s.db.Delete(&series).Error
}

func (s *Service) DeleteTorrent(hash string) error {
	return (&persistence.TorrentModel{Hash: hash}).Delete(s.db)
}

func (s *Service) SearchTorrents(query string) ([]*catalogdomain.TorrentSearchResult, error) {
	log.Printf("Searching for torrents with query: %s", query)
	results, err := s.torrentSearch.Search(query)
	if err != nil {
		return nil, err
	}

	for _, r := range results {
		if err := s.searchCache.Set(r.Hash, r); err != nil {
			return nil, err
		}
	}

	log.Printf("Search completed with %d results", len(results))
	return results, nil
}

type ServiceOptions struct {
	SearchCache   *cache.Cache[*catalogdomain.TorrentSearchResult]
	TorrentEngine domain.ITorrentEngine
	TorrentSearch catalogdomain.ITorrentSearch
	DB            *gorm.DB
}

func NewService(options *ServiceOptions) *Service {
	return &Service{
		searchCache:   options.SearchCache,
		torrentEngine: options.TorrentEngine,
		torrentSearch: options.TorrentSearch,
		db:            options.DB,
	}
}
