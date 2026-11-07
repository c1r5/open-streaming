package http

import (
	"errors"
	stdhttp "net/http"
	"strconv"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/application"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/shared/config"
	sharedhttp "github.com/c1r5/open-streaming/src/shared/http"
	"gorm.io/gorm"
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

		status, code, err := c.service.BeginAdd(&body)
		if err != nil {
			sharedhttp.ErrorJSON(w, code, err.Error())
			return
		}
		sharedhttp.JSON(w, code, status)
	}
}

func (c *Controller) StatusHandler() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		status, err := c.service.TorrentStatus(sharedhttp.PathParam(r, "hash"))
		if err != nil {
			sharedhttp.ErrorJSON(w, application.StatusCode(err), err.Error())
			return
		}
		sharedhttp.JSON(w, stdhttp.StatusOK, status)
	}
}

func (c *Controller) DeleteMovieHandler() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		c.deleteByID(w, r, "movie not found", c.service.DeleteMovie)
	}
}

func (c *Controller) DeleteSeriesHandler() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		c.deleteByID(w, r, "series not found", c.service.DeleteSeries)
	}
}

func (c *Controller) DeleteEpisodeHandler() sharedhttp.HandlerFunc {
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
		err = c.service.DeleteEpisode(uint(seriesID), uint(episodeID))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sharedhttp.ErrorJSON(w, stdhttp.StatusNotFound, "episode not found")
			return
		}
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusInternalServerError, err.Error())
			return
		}
		w.WriteHeader(stdhttp.StatusNoContent)
	}
}

func (c *Controller) deleteByID(w stdhttp.ResponseWriter, r *stdhttp.Request, missing string, remove func(uint) error) {
	id, err := strconv.ParseUint(sharedhttp.PathParam(r, "id"), 10, 64)
	if err != nil {
		sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
		return
	}
	err = remove(uint(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		sharedhttp.ErrorJSON(w, stdhttp.StatusNotFound, missing)
		return
	}
	if err != nil {
		sharedhttp.ErrorJSON(w, stdhttp.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(stdhttp.StatusNoContent)
}

func (c *Controller) MoviesHandler() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		items, err := c.service.ListMovies()
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusInternalServerError, err.Error())
			return
		}
		sharedhttp.JSON(w, stdhttp.StatusOK, items)
	}
}

func (c *Controller) SeriesHandler() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		items, err := c.service.ListSeries()
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusInternalServerError, err.Error())
			return
		}
		sharedhttp.JSON(w, stdhttp.StatusOK, items)
	}
}

func (c *Controller) EpisodesHandler() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		id, err := strconv.ParseUint(sharedhttp.PathParam(r, "id"), 10, 64)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
			return
		}
		items, err := c.service.ListEpisodes(uint(id))
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sharedhttp.ErrorJSON(w, stdhttp.StatusNotFound, "series not found")
			return
		}
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
