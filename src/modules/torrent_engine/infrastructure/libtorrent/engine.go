package libtorrent

import (
	"fmt"
	"log"
	"time"

	libtorrent "github.com/anacrolix/torrent"
	"github.com/c1r5/open-streaming/src/modules/catalog/infrastructure/imdb"
	"github.com/c1r5/open-streaming/src/modules/catalog/infrastructure/torrentio"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/cache"
)

type TorrentEngine struct {
	client         *libtorrent.Client
	imdb           *imdb.IMDB
	torrentio      *torrentio.TorrentioAPI
	cache          *cache.Cache[*TorrentMeta]
	fileCache      *cache.Cache[*resolvedFile]
	dataDir        string
	readaheadBytes int
}

type EngineOptions struct {
	DataDir        string
	ReadaheadMB    int
	CacheTTLMinutes int
}

func NewEngine(imdb *imdb.IMDB, torrentio *torrentio.TorrentioAPI, opts EngineOptions) (domain.ITorrentEngine, error) {
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
