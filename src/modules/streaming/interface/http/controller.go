package http

import (
	"log"
	stdhttp "net/http"
	"strconv"
	"time"

	"github.com/c1r5/open-streaming/src/modules/streaming/application"
	sharedhttp "github.com/c1r5/open-streaming/src/shared/http"
)

type Controller struct {
	service *application.Service
}

func (c *Controller) StreamWatch() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		id, err := strconv.ParseUint(sharedhttp.PathParam(r, "id"), 10, 64)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
			return
		}

		log.Printf("watch: request id=%d method=%s range=%q", id, r.Method, r.Header.Get("Range"))
		media, err := c.service.GetStreamFile(uint(id))
		if err != nil {
			log.Printf("watch: resolve failed id=%d err=%v", id, err)
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

		w.Header().Set("transferMode.dlna.org", "Streaming")
		w.Header().Set("contentFeatures.dlna.org",
			"DLNA.ORG_OP=01;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=01700000000000000000000000000000")
		w.Header().Set("Cache-Control", "max-age=0, no-cache")
		stdhttp.ServeContent(w, r, media.Path, time.Time{}, media.Reader)
	}
}

func (c *Controller) GetStreamInfo() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		id, err := strconv.ParseUint(sharedhttp.PathParam(r, "id"), 10, 64)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
			return
		}

		media, err := c.service.GetStreamInfo(uint(id))
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusNotFound, err.Error())
			return
		}

		sharedhttp.JSON(w, stdhttp.StatusOK, map[string]any{
			"name": media.Filename,
			"hash": media.Hash,
			"imdb": media.IMDB,
			"size": media.Size,
		})
	}
}

func NewController(service *application.Service) *Controller {
	return &Controller{service: service}
}
