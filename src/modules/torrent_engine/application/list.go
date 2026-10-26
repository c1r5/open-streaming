package application

import (
	"sort"
	"time"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
)

type ListedEpisode struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type ListedTorrent struct {
	ID        uint            `json:"id"`
	Name      string          `json:"name"`
	Size      int64           `json:"size"`
	CreatedAt time.Time       `json:"created_at"`
	IMDB      string          `json:"imdb"`
	Episodes  []ListedEpisode `json:"episodes,omitempty"`
}

func (s *Service) ListTorrents() ([]ListedTorrent, error) {
	var movies []persistence.MovieModel
	if err := s.db.Preload("Torrent").Find(&movies).Error; err != nil {
		return nil, err
	}
	var series []persistence.SeriesModel
	if err := s.db.Preload("Torrent").Preload("Episodes").Find(&series).Error; err != nil {
		return nil, err
	}

	items := make([]ListedTorrent, 0, len(movies)+len(series))
	for _, movie := range movies {
		imdb := ""
		if movie.IMDB != nil {
			imdb = *movie.IMDB
		}
		items = append(items, ListedTorrent{
			ID:        domain.MovieWatchID(movie.ID),
			Name:      movie.Torrent.Name,
			Size:      movie.Size,
			CreatedAt: movie.Torrent.CreatedAt,
			IMDB:      imdb,
		})
	}
	for _, item := range series {
		imdb := ""
		if item.IMDB != nil {
			imdb = *item.IMDB
		}
		episodes := episodesOf(item.Episodes)
		id := uint(0)
		if len(episodes) > 0 {
			id = episodes[0].ID
		}
		items = append(items, ListedTorrent{
			ID:        id,
			Name:      item.Torrent.Name,
			Size:      item.Torrent.Size,
			CreatedAt: item.Torrent.CreatedAt,
			IMDB:      imdb,
			Episodes:  episodes,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
}

func episodesOf(rows []persistence.EpisodeModel) []ListedEpisode {
	videos := make([]domain.VideoMeta, 0, len(rows))
	byIndex := make(map[int]persistence.EpisodeModel, len(rows))
	for _, row := range rows {
		videos = append(videos, domain.VideoMeta{Index: row.FileIndex, Name: row.Name, Size: row.Size})
		byIndex[row.FileIndex] = row
	}
	domain.SortVideos(videos)
	episodes := make([]ListedEpisode, 0, len(videos))
	for _, video := range videos {
		row := byIndex[video.Index]
		episodes = append(episodes, ListedEpisode{
			ID:   domain.EpisodeWatchID(row.ID),
			Name: row.Name,
			Size: row.Size,
		})
	}
	return episodes
}
