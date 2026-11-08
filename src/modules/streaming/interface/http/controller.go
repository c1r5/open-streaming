package http

import (
	"errors"
	"io"
	"log"
	"mime/multipart"
	stdhttp "net/http"
	"os"
	"path"
	"path/filepath"
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

func (c *Controller) MovieMedia() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		id, err := strconv.ParseUint(sharedhttp.PathParam(r, "id"), 10, 64)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
			return
		}
		media, err := c.service.MovieMedia(uint(id))
		writeMedia(w, media, err)
	}
}

func (c *Controller) EpisodeMedia() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		seriesID, episodeID, ok := seriesEpisodeIDs(w, r)
		if !ok {
			return
		}
		media, err := c.service.EpisodeMedia(seriesID, episodeID)
		writeMedia(w, media, err)
	}
}

func writeMedia(w stdhttp.ResponseWriter, media *application.Media, err error) {
	if err != nil {
		writeTrackError(w, err)
		return
	}
	sharedhttp.JSON(w, stdhttp.StatusOK, media)
}

func (c *Controller) MovieTracks() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		id, err := strconv.ParseUint(sharedhttp.PathParam(r, "id"), 10, 64)
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
			return
		}
		if r.Method == stdhttp.MethodGet {
			tracks, err := c.service.ListMovieTracks(uint(id))
			writeTracks(w, tracks, err, stdhttp.StatusOK)
			return
		}
		incoming, err := readTracks(w, r)
		if err != nil {
			return
		}
		tracks, err := c.service.AddMovieTracks(uint(id), incoming)
		writeTracks(w, tracks, err, stdhttp.StatusCreated)
	}
}

func (c *Controller) EpisodeTracks() sharedhttp.HandlerFunc {
	return func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		seriesID, episodeID, ok := seriesEpisodeIDs(w, r)
		if !ok {
			return
		}
		if r.Method == stdhttp.MethodGet {
			tracks, err := c.service.ListEpisodeTracks(seriesID, episodeID)
			writeTracks(w, tracks, err, stdhttp.StatusOK)
			return
		}
		incoming, err := readTracks(w, r)
		if err != nil {
			return
		}
		tracks, err := c.service.AddEpisodeTracks(seriesID, episodeID, incoming)
		writeTracks(w, tracks, err, stdhttp.StatusCreated)
	}
}

func seriesEpisodeIDs(w stdhttp.ResponseWriter, r *stdhttp.Request) (uint, uint, bool) {
	seriesID, err := strconv.ParseUint(sharedhttp.PathParam(r, "id"), 10, 64)
	if err != nil {
		sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
		return 0, 0, false
	}
	episodeID, err := strconv.ParseUint(sharedhttp.PathParam(r, "episodeId"), 10, 64)
	if err != nil {
		sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "invalid id")
		return 0, 0, false
	}
	return uint(seriesID), uint(episodeID), true
}

func writeTracks(w stdhttp.ResponseWriter, tracks []application.ListedTrack, err error, status int) {
	if err != nil {
		writeTrackError(w, err)
		return
	}
	if tracks == nil {
		tracks = []application.ListedTrack{}
	}
	sharedhttp.JSON(w, status, map[string]any{"tracks": tracks})
}

func writeTrackError(w stdhttp.ResponseWriter, err error) {
	var status *application.StatusError
	if errors.As(err, &status) {
		sharedhttp.ErrorJSON(w, status.Status, status.Message)
		return
	}
	sharedhttp.ErrorJSON(w, stdhttp.StatusInternalServerError, err.Error())
}

func readTracks(w stdhttp.ResponseWriter, r *stdhttp.Request) ([]application.IncomingTrack, error) {
	r.Body = stdhttp.MaxBytesReader(w, r.Body, 2<<30+32<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "streaming -> track file required")
		return nil, err
	}
	var incoming []application.IncomingTrack
	for _, kind := range []string{"audio", "subtitle"} {
		file, header, err := r.FormFile(kind)
		if err != nil {
			continue
		}
		track, err := storeUpload(kind, file, header, r)
		file.Close()
		if err != nil {
			sharedhttp.ErrorJSON(w, stdhttp.StatusBadRequest, "streaming -> track file required")
			return nil, err
		}
		incoming = append(incoming, track)
	}
	return incoming, nil
}

func storeUpload(kind string, file multipart.File, header *multipart.FileHeader, r *stdhttp.Request) (application.IncomingTrack, error) {
	ext := filepath.Ext(header.Filename)
	tmp, err := os.CreateTemp("", "track-*"+ext)
	if err != nil {
		return application.IncomingTrack{}, err
	}
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return application.IncomingTrack{}, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return application.IncomingTrack{}, err
	}
	return application.IncomingTrack{
		Kind:     kind,
		Name:     header.Filename,
		Path:     tmp.Name(),
		Language: strings.ToLower(strings.TrimSpace(r.FormValue(kind + "_language"))),
		Title:    strings.TrimSpace(r.FormValue(kind + "_title")),
		Default:  formDefault(r.FormValue(kind + "_default")),
	}, nil
}

func formDefault(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on":
		return true
	default:
		return false
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
