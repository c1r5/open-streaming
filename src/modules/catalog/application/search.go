package application

import (
	"fmt"
	"log"

	"github.com/c1r5/open-streaming/src/modules/catalog/domain"
	"github.com/c1r5/open-streaming/src/modules/catalog/infrastructure/imdb"
	"github.com/c1r5/open-streaming/src/modules/catalog/infrastructure/torrentio"
)

type TorrentSearch struct {
	torrentio *torrentio.TorrentioAPI
	imdb      *imdb.IMDB
}

func CreateTorrentSearch(torrentio *torrentio.TorrentioAPI, imdb *imdb.IMDB) *TorrentSearch {
	return &TorrentSearch{
		torrentio: torrentio,
		imdb:      imdb,
	}
}

func (ts *TorrentSearch) Search(query string) ([]*domain.TorrentSearchResult, error) {
	imdbResult, err := ts.imdb.Search(query)
	if err != nil {
		return nil, fmt.Errorf("torrent_engine: search error on %w", err)
	}

	var results []*domain.TorrentSearchResult

	for _, t := range imdbResult {
		streams, err := ts.torrentio.Search(t.Id)
		if err != nil {
			log.Printf("torrent_engine: search error on %s", err.Error())
			continue
		}

		for _, stream := range streams {
			results = append(results, &domain.TorrentSearchResult{
				TorrentName: stream.Title,
				Hash:        stream.InfoHash,
				Year:        t.Year,
				FileIndex:   stream.FileIdx,
				Trackers:    stream.Sources,
				IMDB:        t.Id,
			})
		}
	}

	return results, nil
}
