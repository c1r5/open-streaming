package streaming

import (
	"net/http"

	"github.com/c1r5/open-streaming/src/common"
	"gorm.io/gorm"
)

type StreamingModule struct {
	c   common.IStreamingController
	mux *http.ServeMux
}

func New(mux *http.ServeMux, eng common.ITorrentEngine, db *gorm.DB) {
	service := &Service{engine: eng, db: db}
	controller := createController(service)
	module := &StreamingModule{controller, mux}
	module.registerRoutes()
}

func (s *StreamingModule) registerRoutes() {
	s.mux.HandleFunc("GET /stream/watch/{id}", s.c.WatchStream())
	s.mux.HandleFunc("GET /stream/info/{id}", s.c.GetStream())
}
