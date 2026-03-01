package search

import (
	"fmt"
	"log"

	"github.com/c1r5/open-streaming/src/api"
	"github.com/c1r5/open-streaming/src/common"
)

type TorrentSearch struct {
	torrentio *api.TorrentioAPI
	imdb      *api.IMDB
}

func CreateTorrentSearch(torrentio *api.TorrentioAPI, imdb *api.IMDB) *TorrentSearch {
	return &TorrentSearch{
		torrentio: torrentio,
		imdb:      imdb,
	}
}

func (ts *TorrentSearch) Search(query string) ([]*common.TorrentSearchResult, error) {
	imdbResult, err := ts.imdb.Search(query)
	if err != nil {
		return nil, fmt.Errorf("torrent_engine: search error on %w", err)
	}

	var results []*common.TorrentSearchResult

	for _, t := range imdbResult {
		streams, err := ts.torrentio.Search(t.Id)
		if err != nil {
			log.Printf("torrent_engine: search error on %s", err.Error())
			continue
		}

		for _, stream := range streams {
			results = append(results, &common.TorrentSearchResult{
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
