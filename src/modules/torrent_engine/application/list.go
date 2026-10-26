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
	for _, row := range largestPerHash(rows) {
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

// largestPerHash keeps the query order and one row per info hash.
// The kept row is the largest video. A tie keeps the lower file index.
func largestPerHash(rows []persistence.TorrentModel) []persistence.TorrentModel {
	best := make(map[string]persistence.TorrentModel, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		current, ok := best[row.Hash]
		if !ok {
			best[row.Hash] = row
			order = append(order, row.Hash)
			continue
		}
		if row.Size > current.Size || (row.Size == current.Size && row.FileIndex < current.FileIndex) {
			best[row.Hash] = row
		}
	}
	out := make([]persistence.TorrentModel, 0, len(order))
	for _, hash := range order {
		out = append(out, best[hash])
	}
	return out
}
