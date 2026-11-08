package application

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/c1r5/open-streaming/src/modules/streaming/infrastructure/ffmpeg"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	"github.com/c1r5/open-streaming/src/shared/config"
	"gorm.io/gorm"
)

const maxTrackTitle = 120

var muxGate sync.Mutex

type StatusError struct {
	Status  int
	Message string
}

func (e *StatusError) Error() string { return e.Message }

type IncomingTrack struct {
	Kind     string
	Name     string
	Path     string
	Language string
	Title    string
	Default  bool
}

type ListedTrack struct {
	ID       uint   `json:"id"`
	Kind     string `json:"kind"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Default  bool   `json:"default"`
}

func (s *Service) ListMovieTracks(id uint) ([]ListedTrack, error) {
	var movie persistence.MovieModel
	if err := s.db.First(&movie, id).Error; err != nil {
		return nil, missing("streaming -> movie not found", err)
	}
	return s.tracksFor(movie.ID, 0)
}

func (s *Service) ListEpisodeTracks(seriesID, episodeID uint) ([]ListedTrack, error) {
	episode, err := s.episode(seriesID, episodeID)
	if err != nil {
		return nil, err
	}
	return s.tracksFor(0, episode.ID)
}

func (s *Service) AddMovieTracks(id uint, incoming []IncomingTrack) ([]ListedTrack, error) {
	if !muxGate.TryLock() {
		return nil, &StatusError{Status: 409, Message: "streaming -> mux busy"}
	}
	defer muxGate.Unlock()

	var movie persistence.MovieModel
	if err := s.db.Preload("Torrent").First(&movie, id).Error; err != nil {
		return nil, missing("streaming -> movie not found", err)
	}
	if err := s.muxInto(movie.Torrent.Hash, movie.FileIndex, movie.LocalPlayback, incoming, func() error {
		return s.db.Model(&movie).Update("LocalPlayback", true).Error
	}, func(size int64) error {
		return s.db.Model(&movie).Update("Size", size).Error
	}, func(track IncomingTrack) error {
		movieID := movie.ID
		return s.insertTrack(&movieID, nil, track)
	}); err != nil {
		return nil, err
	}
	return s.tracksFor(movie.ID, 0)
}

func (s *Service) AddEpisodeTracks(seriesID, episodeID uint, incoming []IncomingTrack) ([]ListedTrack, error) {
	if !muxGate.TryLock() {
		return nil, &StatusError{Status: 409, Message: "streaming -> mux busy"}
	}
	defer muxGate.Unlock()

	episode, err := s.episode(seriesID, episodeID)
	if err != nil {
		return nil, err
	}
	if err := s.muxInto(episode.Series.Torrent.Hash, episode.FileIndex, episode.LocalPlayback, incoming, func() error {
		return s.db.Model(&episode).Update("LocalPlayback", true).Error
	}, func(size int64) error {
		return s.db.Model(&episode).Update("Size", size).Error
	}, func(track IncomingTrack) error {
		episodeID := episode.ID
		return s.insertTrack(nil, &episodeID, track)
	}); err != nil {
		return nil, err
	}
	return s.tracksFor(0, episode.ID)
}

func (s *Service) episode(seriesID, episodeID uint) (persistence.EpisodeModel, error) {
	var episode persistence.EpisodeModel
	err := s.db.Preload("Series.Torrent").Where("series_id = ?", seriesID).First(&episode, episodeID).Error
	if err != nil {
		return episode, missing("streaming -> episode not found", err)
	}
	return episode, nil
}

func (s *Service) muxInto(hash string, index int, local bool, incoming []IncomingTrack, mark func() error, save func(int64) error, insert func(IncomingTrack) error) error {
	defer func() {
		for _, track := range incoming {
			os.Remove(track.Path)
		}
	}()
	if len(incoming) == 0 {
		return &StatusError{Status: 400, Message: "streaming -> track file required"}
	}
	for _, track := range incoming {
		if err := validateTrack(track); err != nil {
			return err
		}
	}
	video, complete, err := s.engine.LocalFile(hash, index)
	if err != nil {
		return missing("streaming -> file not found", err)
	}
	if !strings.EqualFold(filepath.Ext(video), ".mkv") {
		return &StatusError{Status: 415, Message: "streaming -> not an mkv"}
	}
	if !local && !complete {
		return &StatusError{Status: 409, Message: "streaming -> video incomplete"}
	}
	if err := s.engine.DetachFile(hash, index); err != nil {
		return err
	}
	if err := mark(); err != nil {
		return err
	}

	timeout := time.Duration(config.Get().Mux.TimeoutSeconds) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	output := filepath.Join(filepath.Dir(video), "."+filepath.Base(video)+".mux-tmp")
	defer os.Remove(output)
	annexes := make([]ffmpeg.MuxTrack, 0, len(incoming))
	for _, track := range incoming {
		annexes = append(annexes, ffmpeg.MuxTrack{
			Path:     track.Path,
			Kind:     track.Kind,
			Language: track.Language,
			Title:    track.Title,
			Default:  track.Default,
		})
	}
	if err := ffmpeg.Mux(ctx, video, annexes, output); err != nil {
		return &StatusError{Status: 422, Message: err.Error()}
	}
	if err := os.Rename(output, video); err != nil {
		return err
	}
	info, err := os.Stat(video)
	if err != nil {
		return err
	}
	if err := save(info.Size()); err != nil {
		return err
	}
	for _, track := range incoming {
		if err := insert(track); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) insertTrack(movieID, episodeID *uint, track IncomingTrack) error {
	query := s.db.Model(&persistence.ExtraTrackModel{}).Where("kind = ?", track.Kind)
	if movieID != nil {
		query = query.Where("movie_id = ?", *movieID)
	} else {
		query = query.Where("episode_id = ?", *episodeID)
	}
	if track.Default {
		if err := query.Update("is_default", false).Error; err != nil {
			return err
		}
	}
	row := persistence.ExtraTrackModel{
		MovieID:   movieID,
		EpisodeID: episodeID,
		Kind:      track.Kind,
		Language:  track.Language,
		Title:     track.Title,
		IsDefault: track.Default,
	}
	return s.db.Create(&row).Error
}

func (s *Service) tracksFor(movieID, episodeID uint) ([]ListedTrack, error) {
	var rows []persistence.ExtraTrackModel
	query := s.db.Order("id")
	if movieID != 0 {
		query = query.Where("movie_id = ?", movieID)
	} else {
		query = query.Where("episode_id = ?", episodeID)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	tracks := make([]ListedTrack, 0, len(rows))
	for _, row := range rows {
		tracks = append(tracks, ListedTrack{
			ID:       row.ID,
			Kind:     row.Kind,
			Language: row.Language,
			Title:    row.Title,
			Default:  row.IsDefault,
		})
	}
	return tracks, nil
}

func validateTrack(track IncomingTrack) error {
	ext := strings.ToLower(filepath.Ext(track.Name))
	switch track.Kind {
	case "audio":
		if !audioExt[ext] {
			return &StatusError{Status: 400, Message: "streaming -> unsupported audio"}
		}
	case "subtitle":
		if !subtitleExt[ext] {
			return &StatusError{Status: 400, Message: "streaming -> unsupported subtitle"}
		}
	default:
		return &StatusError{Status: 400, Message: "streaming -> track file required"}
	}
	if !languageCode.MatchString(track.Language) {
		return &StatusError{Status: 400, Message: "streaming -> invalid language"}
	}
	if strings.ContainsAny(track.Title, "\r\n\x00") || utf8.RuneCountInString(track.Title) > maxTrackTitle {
		return &StatusError{Status: 400, Message: "streaming -> invalid title"}
	}
	return nil
}

func missing(message string, err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) || err != nil {
		return &StatusError{Status: 404, Message: message}
	}
	return nil
}

var languageCode = regexp.MustCompile(`^[a-z]{3}$`)

var audioExt = map[string]bool{
	".aac": true, ".ac3": true, ".eac3": true, ".dts": true, ".flac": true,
	".m4a": true, ".mka": true, ".mp3": true, ".ogg": true, ".opus": true, ".wav": true,
}

var subtitleExt = map[string]bool{
	".srt": true, ".vtt": true, ".ass": true, ".ssa": true,
}
