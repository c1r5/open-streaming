package config

import (
	"fmt"
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
}

type IMDBConfig struct {
	BaseURL string
}

type TorrentioConfig struct {
	BaseURL   string
	UserAgent string
}

type TorrentEngineConfig struct {
	DataDir        string
	ReadaheadMB    int
	CacheTTLMinutes int
}

type TranscodeConfig struct {
	MaxConcurrent    int
	TimeoutSeconds   int
	Preset           string
	CRF              string
	VideoProfile     string
	VideoLevel       string
	AudioBitrate     string
	AudioChannels    int
	ProbeTimeoutSeconds  int
	ProbeSize            string
	AnalyzeDuration      string
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

var global *Config

func init() {
	cfg, err := Load(filepath.Join(binDir, "config.ini"))
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	global = cfg
}

func Get() *Config {
	return global
}

func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := writeDefaults(path); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		log.Printf("config: created default config at %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	sections, err := parseINI(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return buildConfig(sections), nil
}

func buildConfig(sections iniSections) *Config {
	get := func(section, key, def string) string {
		if s, ok := sections[section]; ok {
			if v, ok := s[key]; ok && v != "" {
				return v
			}
		}
		return def
	}

	parseInt := func(section, key string, def int) int {
		v := get(section, key, "")
		if v == "" {
			return def
		}
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return def
		}
		return n
	}

	defaultCFG := &Config{
		Torrentio: TorrentioConfig{
			BaseURL:   get("torrentio", "base_url", os.Getenv("TORRENTIO_BASE_URL")),
			// Use a lib to randomize user_agent 
			UserAgent: get("torrentio", "user_agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36"),
		},
		IMDB: IMDBConfig{
			BaseURL: get("imdb", "base_url", os.Getenv("IMDB_BASE_URL")),
		},
		TorrentEngine: TorrentEngineConfig{
			DataDir:         get("webtorrent", "data_dir", os.Getenv("WEBTORRENT_DATA_DIR")),
			ReadaheadMB:     parseInt("webtorrent", "readahead_mb", 50),
			CacheTTLMinutes: parseInt("webtorrent", "cache_ttl_minutes", 300),
		},
		Transcode: TranscodeConfig{
			MaxConcurrent:       parseInt("transcode", "max_concurrent", 3),
			TimeoutSeconds:      parseInt("transcode", "timeout_seconds", 60),
			Preset:              get("transcode", "preset", "ultrafast"),
			CRF:                 get("transcode", "crf", "23"),
			VideoProfile:        get("transcode", "video_profile", "main"),
			VideoLevel:          get("transcode", "video_level", "4.0"),
			AudioBitrate:        get("transcode", "audio_bitrate", "128k"),
			AudioChannels:       parseInt("transcode", "audio_channels", 2),
			ProbeTimeoutSeconds: parseInt("transcode", "probe_timeout_seconds", 120),
			ProbeSize:           get("transcode", "probe_size", "100M"),
			AnalyzeDuration:     get("transcode", "analyze_duration", "100M"),
		},
		Resolver: ResolverConfig{
			HeadTimeoutSeconds: parseInt("resolver", "head_timeout_seconds", 10),
		},
		Server: ServerConfig{
			Port:               get("server", "port", os.Getenv("SERVER_PORT")),
			ReadTimeoutSeconds: parseInt("server", "read_timeout_seconds", 10),
			MaxHeaderBytes:     parseInt("server", "max_header_bytes", 1<<20),
		},
		Torrent: TorrentConfig{
			SearchCacheTTLMinutes: parseInt("torrent", "search_cache_ttl_minutes", 5),
			AddTimeoutSeconds:     parseInt("torrent", "add_timeout_seconds", 15),
		},
		Database: DatabaseConfig{
			DSN: get("database", "dsn", os.Getenv("DATABASE_DSN")),
		},
	}

	if defaultCFG.Database.DSN == "" {
		defaultCFG.Database.DSN = filepath.Join(binDir, "database.db")
	}

	if os.Getenv("ENVMODE") == envTypeName[DEV] {
		defaultCFG.Server.Port = "3001"
	}

	return defaultCFG
}

func writeDefaults(path string) error {
	// Only create the directory if it is not the current working directory
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(defaultINI), 0o644)
}

const defaultINI = `
[torrentio]
base_url = https://torrentio.strem.fun
user_agent =

[imdb]
base_url = http://api.imdbapi.dev

[webtorrent]
data_dir =
readahead_mb = 50
cache_ttl_minutes = 300

[transcode]
max_concurrent = 3
timeout_seconds = 60
preset = ultrafast
crf = 23
video_profile = main
video_level = 4.0
audio_bitrate = 128k
audio_channels = 2
probe_timeout_seconds = 120
probe_size = 100M
analyze_duration = 100M

[resolver]
head_timeout_seconds = 10

[server]
port = 3000
read_timeout_seconds = 10
max_header_bytes = 1048576

[torrent]
search_cache_ttl_minutes = 5
add_timeout_seconds = 15

[database]
dsn =
`
