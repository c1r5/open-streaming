package libtorrent

import (
	"path/filepath"
	"strings"
)

var videoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true,
	".mov": true, ".m4v": true, ".ts": true,
	".wmv": true, ".flv": true, ".webm": true,
}

func isVideo(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return videoExtensions[ext]
}
