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

type ListedMovie struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	IMDB      string    `json:"imdb"`
}

type ListedSeries struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	IMDB      string    `json:"imdb"`
}

func (s *Service) ListMovies() ([]ListedMovie, error) {
	var movies []persistence.MovieModel
	if err := s.db.Preload("Torrent").Find(&movies).Error; err != nil {
		return nil, err
	}
	items := make([]ListedMovie, 0, len(movies))
	for _, movie := range movies {
		imdb := ""
		if movie.IMDB != nil {
			imdb = *movie.IMDB
		}
		items = append(items, ListedMovie{
			ID:        movie.ID,
			Name:      movie.Torrent.Name,
			Size:      movie.Size,
			CreatedAt: movie.Torrent.CreatedAt,
			IMDB:      imdb,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
}

func (s *Service) ListSeries() ([]ListedSeries, error) {
	var series []persistence.SeriesModel
	if err := s.db.Preload("Torrent").Find(&series).Error; err != nil {
		return nil, err
	}
	items := make([]ListedSeries, 0, len(series))
	for _, item := range series {
		imdb := ""
		if item.IMDB != nil {
			imdb = *item.IMDB
		}
		items = append(items, ListedSeries{
			ID:        item.ID,
			Name:      item.Torrent.Name,
			Size:      item.Torrent.Size,
			CreatedAt: item.Torrent.CreatedAt,
			IMDB:      imdb,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
}

func (s *Service) ListEpisodes(seriesID uint) ([]ListedEpisode, error) {
	var series persistence.SeriesModel
	if err := s.db.First(&series, seriesID).Error; err != nil {
		return nil, err
	}
	var rows []persistence.EpisodeModel
	if err := s.db.Where("series_id = ?", series.ID).Find(&rows).Error; err != nil {
		return nil, err
	}
	return episodesOf(rows), nil
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
			ID:   row.ID,
			Name: row.Name,
			Size: row.Size,
		})
	}
	return episodes
}
