package torrentio

import (
	"testing"
)

func TestTorrentioService_Search(t *testing.T) {
	imdb_id := "tt4649466"
	torrentio_service := CreateTorrentio("https://torrentio.strem.fun/brazuca/", "test-agent")
	streams, err := torrentio_service.Search(imdb_id)
	if err != nil {
		t.Error(err)
	}
	if len(streams) < 1 {
		t.Error("empty_streams")
	}
}
