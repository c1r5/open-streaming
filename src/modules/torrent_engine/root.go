package torrent_engine

import (
	"errors"
	"log"
	"net/http"
	"time"

	catalogdomain "github.com/c1r5/open-streaming/src/modules/catalog/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/application"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/cache"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	torrent_http "github.com/c1r5/open-streaming/src/modules/torrent_engine/interface/http"
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
		torrentID := t.ID
		go func() {
			var series persistence.SeriesModel
			err := db.Where("torrent_id = ?", torrentID).First(&series).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("engine warm-up: series query failed for hash=%s: %v", hash, err)
				return
			}
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if _, err := eng.Resolve(hash); err != nil {
					log.Printf("engine warm-up: failed for hash=%s: %v", hash, err)
					return
				}
				log.Printf("engine warm-up: ready (hash=%s)", hash)
				if err := eng.PersistTorrentFile(hash); err != nil {
					log.Printf("warn: could not persist .torrent for hash=%s: %v", hash, err)
				}
				return
			}

			files, err := eng.ListFiles(&domain.MagnetModel{Hash: hash})
			if err != nil {
				log.Printf("engine warm-up: file list failed for hash=%s: %v", hash, err)
				return
			}
			var episodes []persistence.EpisodeModel
			if err := db.Where("series_id = ?", series.ID).Find(&episodes).Error; err != nil {
				log.Printf("engine warm-up: episode query failed for hash=%s: %v", hash, err)
				return
			}
			keep := map[int]bool{}
			for _, episode := range episodes {
				keep[episode.FileIndex] = true
				if episode.SubtitleIndex != nil {
					keep[*episode.SubtitleIndex] = true
				}
			}
			ignore := make([]int, 0)
			for _, file := range files {
				if !keep[file.Index] {
					ignore = append(ignore, file.Index)
				}
			}
			if err := eng.IgnoreFiles(hash, ignore); err != nil {
				log.Printf("engine warm-up: ignore files failed for hash=%s: %v", hash, err)
			}
			log.Printf("engine warm-up: ready (hash=%s)", hash)
			if err := eng.PersistTorrentFile(hash); err != nil {
				log.Printf("warn: could not persist .torrent for hash=%s: %v", hash, err)
			}
			if err := application.LinkSeriesSubtitles(db, hash, files); err != nil {
				log.Printf("engine warm-up: subtitle link failed for hash=%s: %v", hash, err)
			}
		}()
	}
}

func (m *TorrentModule) registerRoutes() {
	m.mux.HandleFunc("GET /movies", func(w http.ResponseWriter, r *http.Request) {
		m.c.MoviesHandler()(w, r)
	})
	m.mux.HandleFunc("GET /series", func(w http.ResponseWriter, r *http.Request) {
		m.c.SeriesHandler()(w, r)
	})
	m.mux.HandleFunc("GET /series/{id}/episodes", func(w http.ResponseWriter, r *http.Request) {
		m.c.EpisodesHandler()(w, r)
	})
	m.mux.HandleFunc("POST /torrent/add", func(w http.ResponseWriter, r *http.Request) {
		m.c.AddHandler()(w, r)
	})
	m.mux.HandleFunc("DELETE /movies/{id}", func(w http.ResponseWriter, r *http.Request) {
		m.c.DeleteMovieHandler()(w, r)
	})
	m.mux.HandleFunc("DELETE /series/{id}", func(w http.ResponseWriter, r *http.Request) {
		m.c.DeleteSeriesHandler()(w, r)
	})
	m.mux.HandleFunc("DELETE /series/{id}/episodes/{episodeId}", func(w http.ResponseWriter, r *http.Request) {
		m.c.DeleteEpisodeHandler()(w, r)
	})
	m.mux.HandleFunc("GET /torrent/search", func(w http.ResponseWriter, r *http.Request) {
		m.c.SearchHandler()(w, r)
	})
}
