package application

import (
	"path/filepath"
	"strings"

	"github.com/c1r5/open-streaming/src/modules/streaming/infrastructure/ffmpeg"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
)

type MediaVideo struct {
	Codec  string `json:"codec"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type MediaAudio struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Channels int    `json:"channels"`
	Default  bool   `json:"default"`
}

type MediaSubtitle struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Default  bool   `json:"default"`
}

type Media struct {
	DurationSeconds float64         `json:"duration_seconds"`
	Video           *MediaVideo     `json:"video"`
	Audios          []MediaAudio    `json:"audios"`
	Subtitles       []MediaSubtitle `json:"subtitles"`
}

func (s *Service) MovieMedia(id uint) (*Media, error) {
	var movie persistence.MovieModel
	if err := s.db.Preload("Torrent").First(&movie, id).Error; err != nil {
		return nil, missing("streaming -> movie not found", err)
	}
	return s.mediaAt(movie.Torrent.Hash, movie.FileIndex, movie.LocalPlayback)
}

func (s *Service) EpisodeMedia(seriesID, episodeID uint) (*Media, error) {
	episode, err := s.episode(seriesID, episodeID)
	if err != nil {
		return nil, err
	}
	return s.mediaAt(episode.Series.Torrent.Hash, episode.FileIndex, episode.LocalPlayback)
}

func (s *Service) mediaAt(hash string, index int, local bool) (*Media, error) {
	path, complete, err := s.engine.LocalFile(hash, index)
	if err != nil {
		return nil, missing("streaming -> file not found", err)
	}
	if !strings.EqualFold(filepath.Ext(path), ".mkv") {
		return nil, &StatusError{Status: 415, Message: "streaming -> not an mkv"}
	}
	if !local && !complete {
		return nil, &StatusError{Status: 409, Message: "streaming -> video incomplete"}
	}
	probed, err := ffmpeg.ProbeMedia(path)
	if err != nil {
		return nil, &StatusError{Status: 422, Message: "streaming -> probe failed"}
	}
	return mediaFromProbe(probed), nil
}

func mediaFromProbe(probed *ffmpeg.EmbeddedMedia) *Media {
	media := &Media{
		DurationSeconds: probed.Duration,
		Audios:          make([]MediaAudio, 0, len(probed.Audios)),
		Subtitles:       make([]MediaSubtitle, 0, len(probed.Subtitles)),
	}
	if probed.Video != nil {
		media.Video = &MediaVideo{
			Codec:  probed.Video.Codec,
			Width:  probed.Video.Width,
			Height: probed.Video.Height,
		}
	}
	for _, stream := range probed.Audios {
		media.Audios = append(media.Audios, MediaAudio{
			Index:    stream.Index,
			Codec:    stream.Codec,
			Language: stream.Language,
			Title:    stream.Title,
			Channels: stream.Channels,
			Default:  stream.Default,
		})
	}
	for _, stream := range probed.Subtitles {
		media.Subtitles = append(media.Subtitles, MediaSubtitle{
			Index:    stream.Index,
			Codec:    stream.Codec,
			Language: stream.Language,
			Title:    stream.Title,
			Default:  stream.Default,
		})
	}
	return media
}
