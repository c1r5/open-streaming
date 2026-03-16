package libtorrent

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	libtorrent "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
)

type TorrentMeta struct {
	Files     []*libtorrent.File
	Info      *metainfo.Info
	InfoBytes []byte // raw bencode bytes from torrent.Metainfo().InfoBytes
}

func (t *TorrentEngine) fetchMeta(magnetModel *domain.MagnetModel) (*TorrentMeta, error) {
	torrentPath := filepath.Join(t.dataDir, magnetModel.Hash+".torrent")

	var torrent *libtorrent.Torrent

	if _, err := os.Stat(torrentPath); err == nil {
		log.Printf("engine: loading from .torrent file (hash=%s)", magnetModel.Hash)
		torrent, err = t.client.AddTorrentFromFile(torrentPath)
		if err != nil {
			return nil, fmt.Errorf("engine: add from file: %w", err)
		}
	} else {
		log.Printf("engine: fetching metadata via magnet (hash=%s)", magnetModel.Hash)
		var err error
		torrent, err = t.client.AddMagnet(magnetModel.Magnet())
		if err != nil {
			return nil, fmt.Errorf("engine: add magnet: %w", err)
		}
		<-torrent.GotInfo()
		log.Printf("engine: metadata fetched (hash=%s)", magnetModel.Hash)
	}

	mi := torrent.Metainfo()
	return &TorrentMeta{
		Files:     torrent.Files(),
		Info:      torrent.Info(),
		InfoBytes: mi.InfoBytes,
	}, nil
}
