package libtorrent

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	libtorrent "github.com/anacrolix/torrent"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
)

// resolvedFile guarda metadados + referência ao *libtorrent.File para criar novos readers.
type resolvedFile struct {
	path   string
	name   string
	size   int64
	index  int
	source *libtorrent.File
}

func (t *TorrentEngine) Resolve(hash string) (*domain.TorrentFile, error) {
	cached, _ := t.fileCache.Get(hash)

	if cached == nil {
		meta, err := t.cache.Retrieve(hash, func() (*TorrentMeta, error) {
			return t.fetchMeta(&domain.MagnetModel{Hash: hash})
		})

		if err != nil {
			return nil, fmt.Errorf("torrent engine: resolve: %w", err)
		}

		if len(meta.Files) == 0 {
			return nil, fmt.Errorf("torrent engine: no files found in torrent (hash=%s)", hash)
		}

		var file *libtorrent.File

		for _, f := range meta.Files {
			if file == nil || f.Length() > file.Length() {
				if isVideo(f.Path()) {
					file = f
				}
			}
		}

		if file == nil {
			return nil, fmt.Errorf("torrent engine: no video files found in torrent (hash=%s)", hash)
		}

		cached = &resolvedFile{
			path:   filepath.Join(t.dataDir, file.Path()),
			name:   file.DisplayPath(),
			size:   file.Length(),
			source: file,
		}

		t.fileCache.Set(hash, cached)
	}

	log.Printf("engine: resolve creating torrent reader (hash=%s, file=%s, size=%d)", hash, cached.name, cached.size)
	reader := cached.source.NewReader()
	reader.SetResponsive()
	reader.SetReadahead(int64(t.readaheadBytes))
	log.Printf("engine: torrent reader ready (readahead=%dMB)", t.readaheadBytes>>20)

	var finalReader io.ReadSeeker = reader

	if f, err := os.Open(cached.path); err == nil {
		log.Printf("engine: using hybrid reader (path=%s, completed=%d/%d)",
			cached.path, cached.source.BytesCompleted(), cached.size)
		finalReader = &hybridReadSeeker{
			local:  f,
			remote: reader,
			source: cached.source,
			size:   cached.size,
		}
	}

	return &domain.TorrentFile{
		Path:   cached.path,
		Name:   cached.name,
		Size:   cached.size,
		Reader: finalReader,
	}, nil
}
