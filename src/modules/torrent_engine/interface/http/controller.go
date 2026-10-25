package http

import (
	"context"
	stdhttp "net/http"
	"time"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/application"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/shared/config"
	sharedhttp "github.com/c1r5/open-streaming/src/shared/http"
)

type Controller struct {
	service *application.Service
}

func (c *Controller) AddHandler() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		var body domain.AddTorrentOptions

		if err := sharedhttp.BindJSON(r, &body); err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, err.Error())
			return
		}

		if body.Magnet == "" && body.Hash == "" {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "hash or magnet required")
			return
		}
		if body.Magnet != "" {
			if _, err := domain.Parse(body.Magnet); err != nil {
				sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, err.Error())
				return
			}
		}

		timeout := time.Duration(config.Get().Torrent.AddTimeoutSeconds) * time.Second
		tctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		type result struct {
			id  uint
			err error
		}
		ch := make(chan result, 1)
		go func() {
			id, err := c.service.AddTorrent(&body)
			ch <- result{id, err}
		}()

		select {
		case res := <-ch:
			if res.err != nil {
				sharedhttp.ErrorJSON(w, stdhttp.StatusInternalServerError, res.err.Error())
				return
			}
			sharedhttp.JSON(w, stdhttp.StatusCreated, map[string]uint{"id": res.id})
		case <-tctx.Done():
			sharedhttp.ErrorJSON(w, stdhttp.StatusGatewayTimeout, "request timed out")
		}
	}
}

func (c *Controller) DeleteHandler() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {}
}

func (c *Controller) ListHandler() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		items, err := c.service.ListTorrents()
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusInternalServerError, err.Error())
			return
		}
		sharedhttp.JSON(w, stdhttp.StatusOK, items)
	}
}

func (c *Controller) SearchHandler() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if !config.SearchEnabled() {
			sharedhttp.ErrorJSON(w, stdhttp.StatusServiceUnavailable, "search disabled")
			return
		}

		query := sharedhttp.QueryParam(r, "q")
		if query == "" {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "query required")
			return
		}

		results, err := c.service.SearchTorrents(query)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusInternalServerError, err.Error())
			return
		}

		if len(results) == 0 {
			sharedhttp.ErrorJSON(w, stdhttp.StatusNotFound, "no results found")
			return
		}
		sharedhttp.JSON(w, stdhttp.StatusOK, results[0:10])
	}
}

func NewController(service *application.Service) *Controller {
	return &Controller{service: service}
}
