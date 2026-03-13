package engine

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	libtorrent "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/c1r5/open-streaming/src/api"
	"github.com/c1r5/open-streaming/src/common"
	"github.com/c1r5/open-streaming/src/pkgs/cache"
)

type TorrentMeta struct {
	Files     []*libtorrent.File
	Info      *metainfo.Info
	InfoBytes []byte // raw bencode bytes from torrent.Metainfo().InfoBytes
}

// resolvedFile guarda metadados + referência ao *libtorrent.File para criar novos readers.
type resolvedFile struct {
	path   string
	name   string
	size   int64
	index  int
	source *libtorrent.File
}

type TorrentEngine struct {
	client         *libtorrent.Client
	imdb           *api.IMDB
	torrentio      *api.TorrentioAPI
	cache          *cache.Cache[*TorrentMeta]
	fileCache      *cache.Cache[*resolvedFile]
	dataDir        string
	readaheadBytes int
}

func (t *TorrentEngine) fetchMeta(magnetModel *common.MagnetModel) (*TorrentMeta, error) {
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
func (t *TorrentEngine) Resolve(hash string) (*common.TorrentFile, error) {
	cached, _ := t.fileCache.Get(hash)

	if cached == nil {
		meta, err := t.cache.Retrieve(hash, func() (*TorrentMeta, error) {
			return t.fetchMeta(&common.MagnetModel{Hash: hash})
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

	return &common.TorrentFile{
		Path:   cached.path,
		Name:   cached.name,
		Size:   cached.size,
		Reader: finalReader,
	}, nil
}

type EngineOptions struct {
	DataDir        string
	ReadaheadMB    int
	CacheTTLMinutes int
}

func CreateTorrentEngine(imdb *api.IMDB, torrentio *api.TorrentioAPI, opts EngineOptions) (common.ITorrentEngine, error) {
	cfg := libtorrent.NewDefaultClientConfig()
	cfg.DataDir = opts.DataDir
	cfg.NoUpload = true
	cfg.Seed = false
	cfg.ListenPort = 0

	client, err := libtorrent.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("webtorrent: create client: %w", err)
	}

	cacheTTL := time.Duration(opts.CacheTTLMinutes) * time.Minute

	log.Printf("webtorrent: client ready (dataDir=%s)", opts.DataDir)

	return &TorrentEngine{
		client:         client,
		cache:          cache.NewCache[*TorrentMeta](cacheTTL),
		fileCache:      cache.NewCache[*resolvedFile](cacheTTL),
		imdb:           imdb,
		torrentio:      torrentio,
		dataDir:        opts.DataDir,
		readaheadBytes: opts.ReadaheadMB << 20,
	}, nil
}
