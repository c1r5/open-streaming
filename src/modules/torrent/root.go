package torrent

import (
	"log"
	"net/http"
	"time"

	"github.com/c1r5/open-streaming/src/common"
	"github.com/c1r5/open-streaming/src/config"
	"github.com/c1r5/open-streaming/src/pkgs/cache"
	"gorm.io/gorm"
)

type TorrentModule struct {
	mux *http.ServeMux
	s   common.ITorrentSearch
	c   common.ITorrentController
	db  *gorm.DB
}

func New(mux *http.ServeMux, s common.ITorrentSearch, eng common.ITorrentEngine, db *gorm.DB) {
	cacheTTL := time.Duration(config.Get().Torrent.SearchCacheTTLMinutes) * time.Minute
	service := NewService(&ServiceOptions{
		SearchCache:   cache.NewCache[*common.TorrentSearchResult](cacheTTL),
		TorrentEngine: eng,
		TorrentSearch: s,
		DB:            db,
	})

	controller := createController(service)
	module := &TorrentModule{mux, s, controller, db}
	module.registerRoutes()

	go warmUpEngine(eng, db)
}

func warmUpEngine(eng common.ITorrentEngine, db *gorm.DB) {
	var torrents []TorrentModel
	if err := db.Find(&torrents).Error; err != nil {
		log.Printf("engine warm-up: DB query failed: %v", err)
		return
	}
	for _, t := range torrents {
		hash := t.Hash
		go func() {
			if _, err := eng.Resolve(hash); err != nil {
				log.Printf("engine warm-up: failed for hash=%s: %v", hash, err)
			} else {
				log.Printf("engine warm-up: ready (hash=%s)", hash)
				if err := eng.PersistTorrentFile(hash); err != nil {
					log.Printf("warn: could not persist .torrent for hash=%s: %v", hash, err)
				}
			}
		}()
	}
}

func (m *TorrentModule) registerRoutes() {
	m.mux.HandleFunc("POST /torrent/add", m.c.AddHandler())
	m.mux.HandleFunc("GET /torrent/delete/{id}", m.c.DeleteHandler())
	m.mux.HandleFunc("GET /torrent/search", m.c.SearchHandler())
}
