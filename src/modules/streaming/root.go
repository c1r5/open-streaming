package streaming

import (
	"net/http"

	"github.com/c1r5/open-streaming/src/modules/streaming/application"
	streaminghttp "github.com/c1r5/open-streaming/src/modules/streaming/interface/http"
	enginedomain "github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"gorm.io/gorm"
)

type StreamingModule struct {
	c   *streaminghttp.Controller
	mux *http.ServeMux
}

func New(mux *http.ServeMux, eng enginedomain.ITorrentEngine, db *gorm.DB) {
	service := application.NewService(eng, db)
	controller := streaminghttp.NewController(service)
	module := &StreamingModule{controller, mux}
	module.registerRoutes()
}

func (s *StreamingModule) registerRoutes() {
	s.mux.HandleFunc("GET /movies/{id}/watch", func(w http.ResponseWriter, r *http.Request) {
		s.c.MovieWatch()(w, r)
	})
	s.mux.HandleFunc("GET /series/{id}/episodes/{episodeId}/watch", func(w http.ResponseWriter, r *http.Request) {
		s.c.EpisodeWatch()(w, r)
	})
	s.mux.HandleFunc("GET /series/{id}/episodes/{episodeId}/subtitle", func(w http.ResponseWriter, r *http.Request) {
		s.c.EpisodeSubtitle()(w, r)
	})
}
