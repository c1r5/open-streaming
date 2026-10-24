package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
)

var binDir string

func init() {
	exe, err := os.Executable()
	if err != nil {
		log.Fatal("failed to resolve executable path:", err)
	}

	binDir = filepath.Dir(exe)
	global = loadFromEnv()
}

func BinDir() string {
	return binDir
}

type Config struct {
	TorrentEngine TorrentEngineConfig
	Transcode     TranscodeConfig
	Resolver      ResolverConfig
	Server        ServerConfig
	IMDB          IMDBConfig
	Torrentio     TorrentioConfig
	Torrent       TorrentConfig
	Database      DatabaseConfig
	DLNA          DLNAConfig
}

type IMDBConfig struct {
	BaseURL string
}

type TorrentioConfig struct {
	BaseURL   string
	UserAgent string
}

type TorrentEngineConfig struct {
	DataDir         string
	ReadaheadMB     int
	CacheTTLMinutes int
}

type TranscodeConfig struct {
	MaxConcurrent       int
	TimeoutSeconds      int
	Preset              string
	CRF                 string
	VideoProfile        string
	VideoLevel          string
	AudioBitrate        string
	AudioChannels       int
	ProbeTimeoutSeconds int
	ProbeSize           string
	AnalyzeDuration     string
}

type ResolverConfig struct {
	HeadTimeoutSeconds int
}

type ServerConfig struct {
	Port               string
	ReadTimeoutSeconds int
	MaxHeaderBytes     int
}

type TorrentConfig struct {
	SearchCacheTTLMinutes int
	AddTimeoutSeconds     int
}

type DatabaseConfig struct {
	DSN string
}

type DLNAConfig struct {
	Enabled      bool
	FriendlyName string
	Port         int
	Interface    string
}

var global *Config

func Get() *Config {
	return global
}

func SearchEnabled() bool {
	cfg := Get()
	return cfg.IMDB.BaseURL != "" && cfg.Torrentio.BaseURL != ""
}

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func loadFromEnv() *Config {
	cfg := &Config{
		Torrentio: TorrentioConfig{
			BaseURL:   envString("TORRENTIO_BASE_URL", ""),
			UserAgent: envString("TORRENTIO_USER_AGENT", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36"),
		},
		IMDB: IMDBConfig{
			BaseURL: envString("IMDB_BASE_URL", ""),
		},
		TorrentEngine: TorrentEngineConfig{
			DataDir:         envString("WEBTORRENT_DATA_DIR", ""),
			ReadaheadMB:     envInt("WEBTORRENT_READAHEAD_MB", 50),
			CacheTTLMinutes: envInt("WEBTORRENT_CACHE_TTL_MINUTES", 300),
		},
		Transcode: TranscodeConfig{
			MaxConcurrent:       envInt("TRANSCODE_MAX_CONCURRENT", 3),
			TimeoutSeconds:      envInt("TRANSCODE_TIMEOUT_SECONDS", 60),
			Preset:              envString("TRANSCODE_PRESET", "ultrafast"),
			CRF:                 envString("TRANSCODE_CRF", "23"),
			VideoProfile:        envString("TRANSCODE_VIDEO_PROFILE", "main"),
			VideoLevel:          envString("TRANSCODE_VIDEO_LEVEL", "4.0"),
			AudioBitrate:        envString("TRANSCODE_AUDIO_BITRATE", "128k"),
			AudioChannels:       envInt("TRANSCODE_AUDIO_CHANNELS", 2),
			ProbeTimeoutSeconds: envInt("TRANSCODE_PROBE_TIMEOUT_SECONDS", 120),
			ProbeSize:           envString("TRANSCODE_PROBE_SIZE", "100M"),
			AnalyzeDuration:     envString("TRANSCODE_ANALYZE_DURATION", "100M"),
		},
		Resolver: ResolverConfig{
			HeadTimeoutSeconds: envInt("RESOLVER_HEAD_TIMEOUT_SECONDS", 10),
		},
		Server: ServerConfig{
			Port:               envString("SERVER_PORT", "3000"),
			ReadTimeoutSeconds: envInt("SERVER_READ_TIMEOUT_SECONDS", 10),
			MaxHeaderBytes:     envInt("SERVER_MAX_HEADER_BYTES", 1<<20),
		},
		Torrent: TorrentConfig{
			SearchCacheTTLMinutes: envInt("TORRENT_SEARCH_CACHE_TTL_MINUTES", 5),
			AddTimeoutSeconds:     envInt("TORRENT_ADD_TIMEOUT_SECONDS", 15),
		},
		Database: DatabaseConfig{
			DSN: envString("DATABASE_DSN", ""),
		},
		DLNA: DLNAConfig{
			Enabled:      envString("DLNA_ENABLED", "true") == "true",
			FriendlyName: envString("DLNA_FRIENDLY_NAME", "Local TV"),
			Port:         envInt("DLNA_PORT", 8200),
			Interface:    envString("DLNA_INTERFACE", ""),
		},
	}

	if cfg.Database.DSN == "" {
		cfg.Database.DSN = filepath.Join(binDir, "database.db")
	}

	if os.Getenv("ENVMODE") == envTypeName[DEV] {
		cfg.Server.Port = "3001"
	}

	return cfg
}
