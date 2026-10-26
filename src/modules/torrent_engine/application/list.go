package application

import (
	"path"
	"strings"
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
	var rows []persistence.TorrentModel
	if err := s.db.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}

	grouped := map[string][]persistence.TorrentModel{}
	order := make([]string, 0)
	for _, row := range rows {
		if _, ok := grouped[row.Hash]; !ok {
			order = append(order, row.Hash)
		}
		grouped[row.Hash] = append(grouped[row.Hash], row)
	}

	items := make([]ListedTorrent, 0, len(order))
	for _, hash := range order {
		group := grouped[hash]
		if isSeriesGroup(group) {
			items = append(items, seriesItem(group))
			continue
		}
		items = append(items, movieItem(group[0]))
	}
	return items, nil
}

func isSeriesGroup(group []persistence.TorrentModel) bool {
	if len(group) > 1 {
		return true
	}
	return len(group) == 1 && domain.HasEpisode(group[0].TorrentName)
}

func movieItem(row persistence.TorrentModel) ListedTorrent {
	imdb := ""
	if row.IMDB != nil {
		imdb = *row.IMDB
	}
	return ListedTorrent{
		ID:        row.ID,
		Name:      row.TorrentName,
		Size:      row.Size,
		CreatedAt: row.CreatedAt,
		IMDB:      imdb,
	}
}

func seriesItem(group []persistence.TorrentModel) ListedTorrent {
	videos := make([]domain.VideoMeta, 0, len(group))
	byIndex := make(map[int]persistence.TorrentModel, len(group))
	var newest time.Time
	imdb := ""
	for _, row := range group {
		videos = append(videos, domain.VideoMeta{
			Index: row.FileIndex,
			Name:  row.TorrentName,
			Size:  row.Size,
		})
		byIndex[row.FileIndex] = row
		if row.CreatedAt.After(newest) {
			newest = row.CreatedAt
		}
		if imdb == "" && row.IMDB != nil {
			imdb = *row.IMDB
		}
	}
	domain.SortVideos(videos)

	episodes := make([]ListedEpisode, 0, len(videos))
	var size int64
	var firstID uint
	for _, video := range videos {
		row := byIndex[video.Index]
		base := path.Base(strings.ReplaceAll(row.TorrentName, "\\", "/"))
		episodes = append(episodes, ListedEpisode{ID: row.ID, Name: base, Size: row.Size})
		size += row.Size
		if firstID == 0 {
			firstID = row.ID
		}
	}
	return ListedTorrent{
		ID:        firstID,
		Name:      domain.PackTitle(videos[0].Name),
		Size:      size,
		CreatedAt: newest,
		IMDB:      imdb,
		Episodes:  episodes,
	}
}
