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
	return t.resolve(&domain.MagnetModel{Hash: hash})
}

func (t *TorrentEngine) ResolveMagnet(magnet *domain.MagnetModel) (*domain.TorrentFile, error) {
	if magnet == nil || magnet.Hash == "" {
		return nil, fmt.Errorf("torrent engine: magnet has no info hash")
	}
	return t.resolve(magnet)
}

func (t *TorrentEngine) ResolveFile(hash string, index int) (*domain.TorrentFile, error) {
	return t.open(&domain.MagnetModel{Hash: hash}, &index)
}

func (t *TorrentEngine) ListVideos(magnet *domain.MagnetModel) ([]domain.VideoMeta, error) {
	if magnet == nil || magnet.Hash == "" {
		return nil, fmt.Errorf("torrent engine: magnet has no info hash")
	}

	meta, err := t.loadMeta(magnet)
	if err != nil {
		return nil, err
	}

	videos := make([]domain.VideoMeta, 0)
	for index, file := range meta.Files {
		if file == nil || !isVideo(file.Path()) {
			continue
		}
		videos = append(videos, domain.VideoMeta{
			Index: index,
			Name:  file.DisplayPath(),
			Path:  filepath.Join(t.dataDir, file.Path()),
			Size:  file.Length(),
		})
	}
	return videos, nil
}

func (t *TorrentEngine) ListFiles(magnet *domain.MagnetModel) ([]domain.VideoMeta, error) {
	if magnet == nil || magnet.Hash == "" {
		return nil, fmt.Errorf("torrent engine: magnet has no info hash")
	}

	meta, err := t.loadMeta(magnet)
	if err != nil {
		return nil, err
	}

	files := make([]domain.VideoMeta, 0, len(meta.Files))
	for index, file := range meta.Files {
		if file == nil {
			continue
		}
		files = append(files, domain.VideoMeta{
			Index: index,
			Name:  file.DisplayPath(),
			Path:  filepath.Join(t.dataDir, file.Path()),
			Size:  file.Length(),
		})
	}
	return files, nil
}

func (t *TorrentEngine) resolve(magnet *domain.MagnetModel) (*domain.TorrentFile, error) {
	return t.open(magnet, nil)
}

func fileCacheKey(hash string, index *int) string {
	if index == nil {
		return hash
	}
	return fmt.Sprintf("%s:%d", hash, *index)
}

func (t *TorrentEngine) open(magnet *domain.MagnetModel, index *int) (*domain.TorrentFile, error) {
	hash := magnet.Hash
	key := fileCacheKey(hash, index)
	cached, _ := t.fileCache.Get(key)

	if cached == nil {
		meta, err := t.loadMeta(magnet)
		if err != nil {
			return nil, err
		}

		if len(meta.Files) == 0 {
			return nil, fmt.Errorf("torrent engine: no files found in torrent (hash=%s)", hash)
		}

		var file *libtorrent.File
		fileIndex := 0

		if index != nil {
			fileIndex = *index
			if fileIndex < 0 || fileIndex >= len(meta.Files) || meta.Files[fileIndex] == nil {
				return nil, fmt.Errorf("torrent engine: file index %d not in torrent (hash=%s)", fileIndex, hash)
			}
			file = meta.Files[fileIndex]
		} else {
			for i, f := range meta.Files {
				if f == nil || !isVideo(f.Path()) || domain.IsSamplePath(f.DisplayPath()) {
					continue
				}
				if file == nil || f.Length() > file.Length() {
					file = f
					fileIndex = i
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
			index:  fileIndex,
			source: file,
		}

		t.fileCache.Set(key, cached)
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
		Index:  cached.index,
		Reader: finalReader,
	}, nil
}

func (t *TorrentEngine) loadMeta(magnet *domain.MagnetModel) (*TorrentMeta, error) {
	meta, err := t.cache.Retrieve(magnet.Hash, func() (*TorrentMeta, error) {
		return t.fetchMeta(magnet)
	})
	if err != nil {
		return nil, fmt.Errorf("torrent engine: resolve: %w", err)
	}
	return meta, nil
}
