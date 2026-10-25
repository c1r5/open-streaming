package application

import (
	"time"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
)

type ListedTorrent struct {
	ID        uint      `json:"id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	IMDB      string    `json:"imdb"`
}

func (s *Service) ListTorrents() ([]ListedTorrent, error) {
	var rows []persistence.TorrentModel
	if err := s.db.Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]ListedTorrent, 0, len(rows))
	for _, row := range rows {
		imdb := ""
		if row.IMDB != nil {
			imdb = *row.IMDB
		}
		items = append(items, ListedTorrent{
			ID:        row.ID,
			Name:      row.TorrentName,
			Size:      row.Size,
			CreatedAt: row.CreatedAt,
			IMDB:      imdb,
		})
	}
	return items, nil
}
