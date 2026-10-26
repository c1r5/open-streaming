package application

import (
	"fmt"
	"log"

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
	if len(videos) != 1 {
		imdb = nil
	}

	if err := s.torrentEngine.PersistTorrentFile(magnet.Hash); err != nil {
		log.Printf("warn: could not persist .torrent for hash=%s: %v", magnet.Hash, err)
	}

	return s.upsertVideos(magnet.Hash, videos, imdb)
}

func (s *Service) upsertVideos(hash string, videos []domain.VideoMeta, imdb *string) (uint, error) {
	var existing []persistence.TorrentModel
	if err := s.db.Where("hash = ?", hash).Find(&existing).Error; err != nil {
		return 0, err
	}

	byIndex := make(map[int]persistence.TorrentModel, len(existing))
	for _, row := range existing {
		byIndex[row.FileIndex] = row
	}

	kept := make(map[uint]bool, len(videos))
	var firstID uint
	for _, video := range videos {
		row, ok := byIndex[video.Index]
		if !ok && len(videos) == 1 && len(existing) > 0 {
			row = existing[0]
			ok = true
		}
		if !ok {
			created := &persistence.TorrentModel{
				IMDB:        imdb,
				Hash:        hash,
				FileIndex:   video.Index,
				TorrentName: video.Name,
				OutputPath:  video.Path,
				Size:        video.Size,
			}
			if err := created.Create(s.db); err != nil {
				return 0, err
			}
			kept[created.ID] = true
			if firstID == 0 {
				firstID = created.ID
			}
			continue
		}

		updates := map[string]any{
			"file_index":   video.Index,
			"torrent_name": video.Name,
			"output":       video.Path,
			"size":         video.Size,
		}
		if imdb != nil {
			updates["imdb"] = *imdb
		}
		if err := s.db.Model(&persistence.TorrentModel{}).Where("id = ?", row.ID).Updates(updates).Error; err != nil {
			return 0, err
		}
		kept[row.ID] = true
		if firstID == 0 {
			firstID = row.ID
		}
	}

	for _, row := range existing {
		if kept[row.ID] {
			continue
		}
		if err := s.db.Delete(&persistence.TorrentModel{}, row.ID).Error; err != nil {
			return 0, err
		}
	}
	return firstID, nil
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
