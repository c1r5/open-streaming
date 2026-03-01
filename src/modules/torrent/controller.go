package torrent

import (
	"context"
	"net/http"
	"time"

	"github.com/c1r5/open-streaming/src/common"
	"github.com/c1r5/open-streaming/src/config"
)

type Controller struct {
	service common.ITorrentService
}

func (c *Controller) AddHandler() common.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body common.AddTorrentOptions

		if err := common.BindJSON(r, &body); err != nil {
			common.ErrorJSON(w, http.StatusBadRequest, err.Error())
			return
		}

		timeout := time.Duration(config.Get().Torrent.AddTimeoutSeconds) * time.Second
		tctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		err_channel := make(chan error, 1)
		go func() { err_channel <- c.service.AddTorrent(&body) }()

		select {
		case err := <-err_channel:
			if err != nil {
				common.ErrorJSON(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.WriteHeader(http.StatusCreated)
		case <-tctx.Done():
			common.ErrorJSON(w, http.StatusGatewayTimeout, "request timed out")
		}
	}
}

func (c *Controller) DeleteHandler() common.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {}
}

func (c *Controller) SearchHandler() common.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := common.QueryParam(r, "q")

		results, err := c.service.SearchTorrents(query)
		if err != nil {
			common.ErrorJSON(w, http.StatusInternalServerError, err.Error())
			return
		}

		if len(results) == 0 {
			common.ErrorJSON(w, http.StatusNotFound, "no results found")
			return
		}
		common.JSON(w, http.StatusOK, results[0:10])
	}
}

func createController(service common.ITorrentService) common.ITorrentController {
	return &Controller{service: service}
}
