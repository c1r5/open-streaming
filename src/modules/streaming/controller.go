package streaming

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/c1r5/open-streaming/src/common"
)

type Controller struct {
	service common.IStreamingService
}

func (c *Controller) WatchStream() common.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseUint(common.PathParam(r, "id"), 10, 64)
		if err != nil {
			common.ErrorJSON(w, http.StatusBadRequest, "invalid id")
			return
		}

		media, err := c.service.GetStream(uint(id))
		if err != nil {
			common.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}

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
		http.ServeContent(w, r, media.Path, time.Time{}, media.Reader)
	}
}

func (c *Controller) GetStream() common.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseUint(common.PathParam(r, "id"), 10, 64)
		if err != nil {
			common.ErrorJSON(w, http.StatusBadRequest, "invalid id")
			return
		}

		media, err := c.service.GetStream(uint(id))
		if err != nil {
			common.ErrorJSON(w, http.StatusNotFound, err.Error())
			return
		}

		common.JSON(w, http.StatusOK, map[string]any{
			"name": media.Name,
			"path": media.Path,
			"size": media.Size,
		})
	}
}

func createController(service common.IStreamingService) common.IStreamingController {
	return &Controller{service: service}
}
