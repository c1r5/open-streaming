package libtorrent

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/anacrolix/torrent/metainfo"
)

func (t *TorrentEngine) PersistTorrentFile(hash string) error {
	meta, err := t.cache.Get(hash)
	if err != nil {
		return fmt.Errorf("engine: persist: hash not in cache: %w", err)
	}

	mi := metainfo.MetaInfo{InfoBytes: meta.InfoBytes}
	path := filepath.Join(t.dataDir, hash+".torrent")

	if _, err := os.Stat(path); err == nil {
		log.Printf("engine: .torrent already exists, skipping (hash=%s)", hash)
		return nil
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("engine: persist: create file: %w", err)
	}
	defer f.Close()

	if err := mi.Write(f); err != nil {
		return fmt.Errorf("engine: persist: write: %w", err)
	}

	log.Printf("engine: .torrent saved (hash=%s, path=%s)", hash, path)
	return nil
}
