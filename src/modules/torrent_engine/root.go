package torrent_engine

import (
	"log"
	"net/http"
	"time"

	catalogdomain "github.com/c1r5/open-streaming/src/modules/catalog/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/application"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/cache"
	torrent_http "github.com/c1r5/open-streaming/src/modules/torrent_engine/interface/http"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	"github.com/c1r5/open-streaming/src/shared/config"
	"gorm.io/gorm"
)

type TorrentModule struct {
	mux *http.ServeMux
	s   catalogdomain.ITorrentSearch
	c   *torrent_http.Controller
	db  *gorm.DB
}

func New(mux *http.ServeMux, s catalogdomain.ITorrentSearch, eng domain.ITorrentEngine, db *gorm.DB) {
	cacheTTL := time.Duration(config.Get().Torrent.SearchCacheTTLMinutes) * time.Minute
	service := application.NewService(&application.ServiceOptions{
		SearchCache:   cache.NewCache[*catalogdomain.TorrentSearchResult](cacheTTL),
		TorrentEngine: eng,
		TorrentSearch: s,
		DB:            db,
	})

	controller := torrent_http.NewController(service)
	module := &TorrentModule{mux, s, controller, db}
	module.registerRoutes()

	go warmUpEngine(eng, db)
}

func warmUpEngine(eng domain.ITorrentEngine, db *gorm.DB) {
	var torrents []persistence.TorrentModel
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
	m.mux.HandleFunc("POST /torrent/add", func(w http.ResponseWriter, r *http.Request) {
		m.c.AddHandler()(w, r)
	})
	m.mux.HandleFunc("GET /torrent/delete/{id}", func(w http.ResponseWriter, r *http.Request) {
		m.c.DeleteHandler()(w, r)
	})
	m.mux.HandleFunc("GET /torrent/search", func(w http.ResponseWriter, r *http.Request) {
		m.c.SearchHandler()(w, r)
	})
}
