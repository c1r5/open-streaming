package http

import (
	"io"
	"log"
	stdhttp "net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/c1r5/open-streaming/src/modules/streaming/application"
	streamdomain "github.com/c1r5/open-streaming/src/modules/streaming/domain"
	sharedhttp "github.com/c1r5/open-streaming/src/shared/http"
)

type Controller struct {
	service *application.Service
}

func (c *Controller) MovieWatch() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		id, err := strconv.ParseUint(sharedhttp.PathParam(r, "id"), 10, 64)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
			return
		}
		log.Printf("watch: movie id=%d method=%s range=%q", id, r.Method, r.Header.Get("Range"))
		media, err := c.service.GetMovieFile(uint(id))
		c.serve(w, r, media, err)
	}
}

func (c *Controller) EpisodeWatch() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		seriesID, err := strconv.ParseUint(sharedhttp.PathParam(r, "id"), 10, 64)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
			return
		}
		episodeID, err := strconv.ParseUint(sharedhttp.PathParam(r, "episodeId"), 10, 64)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
			return
		}
		log.Printf("watch: series=%d episode=%d method=%s range=%q", seriesID, episodeID, r.Method, r.Header.Get("Range"))
		media, err := c.service.GetEpisodeFile(uint(seriesID), uint(episodeID))
		c.serve(w, r, media, err)
	}
}

func (c *Controller) EpisodeSubtitle() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		seriesID, err := strconv.ParseUint(sharedhttp.PathParam(r, "id"), 10, 64)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
			return
		}
		episodeID, err := strconv.ParseUint(sharedhttp.PathParam(r, "episodeId"), 10, 64)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
			return
		}
		media, err := c.service.GetEpisodeSubtitle(uint(seriesID), uint(episodeID))
		if err == nil && r.URL.Query().Get("format") == "vtt" {
			c.serveVTT(w, media)
			return
		}
		c.serve(w, r, media, err)
	}
}

func (c *Controller) serveVTT(w stdhttp.ResponseWriter, media *streamdomain.TorrentStreamFile) {
	defer closeReader(media)
	body, err := io.ReadAll(media.Reader)
	if err != nil {
		sharedhttp.ErrorJSON(w, stdhttp.StatusNotFound, err.Error())
		return
	}
	switch strings.ToLower(path.Ext(media.Path)) {
	case ".srt":
		body = []byte(application.SRTToVTT(string(body)))
	case ".vtt":
	default:
		sharedhttp.ErrorJSON(w, stdhttp.StatusNotFound, "streaming -> subtitle not found")
		return
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	w.WriteHeader(stdhttp.StatusOK)
	_, _ = w.Write(body)
}

func closeReader(media *streamdomain.TorrentStreamFile) {
	if media == nil {
		return
	}
	if closer, ok := media.Reader.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func (c *Controller) serve(w stdhttp.ResponseWriter, r *stdhttp.Request, media *streamdomain.TorrentStreamFile, err error) {
	if err != nil {
		log.Printf("watch: resolve failed err=%v", err)
		sharedhttp.ErrorJSON(w, stdhttp.StatusNotFound, err.Error())
		return
	}
	log.Printf("watch: serving file=%s", media.Path)

	defer func() {
		if closer, ok := media.Reader.(interface{ Close() error }); ok {
			if closeErr := closer.Close(); closeErr != nil {
				log.Printf("watch: reader close error id=%s err=%v", media.Path, closeErr)
			} else {
				log.Printf("watch: reader closed id=%s", media.Path)
			}
		}
	}()

	if media.ContentType != "" {
		w.Header().Set("Content-Type", media.ContentType)
	}
	w.Header().Set("transferMode.dlna.org", "Streaming")
	w.Header().Set("contentFeatures.dlna.org",
		"DLNA.ORG_OP=01;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=01700000000000000000000000000000")
	w.Header().Set("Cache-Control", "max-age=0, no-cache")
	stdhttp.ServeContent(w, r, media.Path, time.Time{}, media.Reader)
}

func NewController(service *application.Service) *Controller {
	return &Controller{service: service}
}
