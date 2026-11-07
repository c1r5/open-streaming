package application

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	"github.com/c1r5/open-streaming/src/shared/config"
	"gorm.io/gorm"
)

const (
	phaseMetadata = "metadata"
	phaseReady    = "ready"
	phaseFailed   = "failed"
)

type FetchStatus struct {
	Hash  string `json:"hash"`
	Phase string `json:"phase"`
	Peers int    `json:"peers"`
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	ID    uint   `json:"id"`
	Error string `json:"error"`
}

type addJob struct {
	gen    int
	status FetchStatus
}

type statusError struct {
	code int
	err  error
}

func (e *statusError) Error() string { return e.err.Error() }

func (e *statusError) Unwrap() error { return e.err }

func fail(code int, err error) error {
	return &statusError{code: code, err: err}
}

func StatusCode(err error) int {
	var status *statusError
	if errors.As(err, &status) {
		return status.code
	}
	return http.StatusInternalServerError
}

func (s *Service) BeginAdd(options *domain.AddTorrentOptions) (FetchStatus, int, error) {
	hash, err := s.prepareAdd(options)
	if err != nil {
		return FetchStatus{}, StatusCode(err), err
	}

	stored, ok, err := s.storedStatus(hash)
	if err != nil {
		return FetchStatus{}, http.StatusInternalServerError, err
	}
	if ok {
		return s.overlay(stored), http.StatusOK, nil
	}

	s.mu.Lock()
	if s.jobs == nil {
		s.jobs = map[string]*addJob{}
	}
	if job, exists := s.jobs[hash]; exists && job.status.Phase == phaseMetadata {
		status := job.status
		s.mu.Unlock()
		return s.overlay(status), http.StatusOK, nil
	}
	gen := 1
	if job, exists := s.jobs[hash]; exists {
		gen = job.gen + 1
	}
	status := FetchStatus{Hash: hash, Phase: phaseMetadata}
	s.jobs[hash] = &addJob{gen: gen, status: status}
	s.mu.Unlock()

	opts := *options
	go s.runAdd(gen, hash, opts)
	return s.overlay(status), http.StatusAccepted, nil
}

func (s *Service) TorrentStatus(hash string) (FetchStatus, error) {
	normalized, ok := normalizeHash(hash)
	if !ok {
		return FetchStatus{}, fail(http.StatusBadRequest, fmt.Errorf("invalid hash"))
	}

	stored, found, err := s.storedStatus(normalized)
	if err != nil {
		return FetchStatus{}, fail(http.StatusInternalServerError, err)
	}
	if found {
		return s.overlay(stored), nil
	}

	s.mu.Lock()
	job, exists := s.jobs[normalized]
	var status FetchStatus
	if exists {
		status = job.status
	}
	s.mu.Unlock()
	if !exists {
		return FetchStatus{}, fail(http.StatusNotFound, fmt.Errorf("torrent not found"))
	}
	return s.overlay(status), nil
}

func (s *Service) prepareAdd(options *domain.AddTorrentOptions) (string, error) {
	if options.Magnet != "" {
		magnet, err := domain.Parse(options.Magnet)
		if err != nil {
			return "", fail(http.StatusBadRequest, err)
		}
		hash, ok := normalizeHash(magnet.Hash)
		if !ok {
			return "", fail(http.StatusBadRequest, fmt.Errorf("magnet: no valid info hash found"))
		}
		return hash, nil
	}
	if options.Hash == "" {
		return "", fail(http.StatusBadRequest, fmt.Errorf("hash or magnet required"))
	}
	hash, ok := normalizeHash(options.Hash)
	if !ok {
		return "", fail(http.StatusBadRequest, fmt.Errorf("invalid hash"))
	}
	if s.searchCache == nil {
		return "", fail(http.StatusBadRequest, fmt.Errorf("failed to get torrent from cache: cache: key not found"))
	}
	if _, err := s.searchCache.Get(options.Hash); err != nil {
		if _, err2 := s.searchCache.Get(hash); err2 != nil {
			return "", fail(http.StatusBadRequest, fmt.Errorf("failed to get torrent from cache: %w", err))
		}
	}
	return hash, nil
}

func (s *Service) runAdd(gen int, hash string, options domain.AddTorrentOptions) {
	timeout := time.Duration(config.Get().Torrent.AddTimeoutSeconds) * time.Second
	timer := time.AfterFunc(timeout, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		job := s.jobs[hash]
		if job == nil || job.gen != gen || job.status.Phase != phaseMetadata {
			return
		}
		job.status.Phase = phaseFailed
		job.status.Error = "request timed out"
	})
	defer timer.Stop()

	kind, id, err := s.AddTorrent(&options)
	s.mu.Lock()
	defer s.mu.Unlock()
	job := s.jobs[hash]
	if job == nil || job.gen != gen {
		return
	}
	if err != nil {
		if job.status.Phase == phaseFailed {
			return
		}
		job.status.Phase = phaseFailed
		job.status.Error = err.Error()
		return
	}
	job.status.Phase = phaseReady
	job.status.Kind = kind
	job.status.ID = id
	job.status.Error = ""
}

func (s *Service) storedStatus(hash string) (FetchStatus, bool, error) {
	var torrent persistence.TorrentModel
	err := s.db.Where("lower(hash) = ?", hash).First(&torrent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return FetchStatus{}, false, nil
	}
	if err != nil {
		return FetchStatus{}, false, err
	}

	status := FetchStatus{Hash: hash, Phase: phaseReady, Name: torrent.Name}
	var movie persistence.MovieModel
	err = s.db.Where("torrent_id = ?", torrent.ID).First(&movie).Error
	if err == nil {
		status.Kind = "movie"
		status.ID = movie.ID
		return status, true, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return FetchStatus{}, false, err
	}

	var series persistence.SeriesModel
	err = s.db.Where("torrent_id = ?", torrent.ID).First(&series).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return FetchStatus{}, false, nil
	}
	if err != nil {
		return FetchStatus{}, false, err
	}
	status.Kind = "series"
	status.ID = series.ID
	return status, true, nil
}

func (s *Service) overlay(status FetchStatus) FetchStatus {
	if s.torrentEngine == nil {
		return status
	}
	view, ok := s.torrentEngine.Swarm(status.Hash)
	if !ok {
		return status
	}
	status.Peers = view.Peers
	if view.Name != "" {
		status.Name = view.Name
	}
	return status
}

func normalizeHash(hash string) (string, bool) {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if len(hash) != 40 && len(hash) != 64 {
		return "", false
	}
	for _, r := range hash {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", false
		}
	}
	return hash, true
}
