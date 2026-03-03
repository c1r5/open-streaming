package torrent

import (
	"errors"
	"fmt"
	"log"

	"github.com/c1r5/open-streaming/src/common"
	"github.com/c1r5/open-streaming/src/pkgs/cache"
	"gorm.io/gorm"
)
type Service struct {
	searchCache   *cache.Cache[*common.TorrentSearchResult]
	torrentEngine common.ITorrentEngine
	torrentSearch common.ITorrentSearch
	db            *gorm.DB
}

func (s *Service) AddTorrent(options *common.AddTorrentOptions) (uint, error) {

	searchResult, err := s.searchCache.Get(options.Hash)

	if err != nil {
		return 0, fmt.Errorf("failed to get torrent from cache: %w", err)
	}

	file, err := s.torrentEngine.Resolve(options.Hash)

	if err != nil {
		return 0, fmt.Errorf("failed to resolve torrent: %w", err)
	}

	if err := s.torrentEngine.PersistTorrentFile(options.Hash); err != nil {
		log.Printf("warn: could not persist .torrent for hash=%s: %v", options.Hash, err)
	}

	model := &TorrentModel{
		IMDB:        searchResult.IMDB,
		Hash:        options.Hash,
		FileIndex:   file.Index,
		TorrentName: file.Name,
		OutputPath:  file.Path,
		Size:        file.Size,
	}
	var existing *TorrentModel
	
	err = s.db.Where("hash = ?", options.Hash).First(&existing).Error

	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}

		if err := model.Create(s.db); err != nil {
			return 0, nil
		}

		return model.ID, nil
	}

	return existing.ID, nil
	
}

func (s *Service) DeleteTorrent(hash string) error {
	return (&TorrentModel{Hash: hash}).Delete(s.db)
}

func (s *Service) SearchTorrents(query string) ([]*common.TorrentSearchResult, error) {
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
	SearchCache   *cache.Cache[*common.TorrentSearchResult]
	TorrentEngine common.ITorrentEngine
	TorrentSearch common.ITorrentSearch
	DB            *gorm.DB
}

func NewService(options *ServiceOptions) common.ITorrentService {
	return &Service{
		searchCache:   options.SearchCache,
		torrentEngine: options.TorrentEngine,
		torrentSearch: options.TorrentSearch,
		db:            options.DB,
	}
}
