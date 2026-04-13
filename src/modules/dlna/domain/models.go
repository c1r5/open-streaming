package domain

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	RootPath   = "/"
	MoviesPath = "/Movies"
)

var videoMIME = map[string]string{
	".mkv":  "video/x-matroska",
	".mp4":  "video/mp4",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",
	".m4v":  "video/x-m4v",
	".ts":   "video/mp2t",
	".wmv":  "video/x-ms-wmv",
	".flv":  "video/x-flv",
	".webm": "video/webm",
}

func MIMEForPath(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if m, ok := videoMIME[ext]; ok {
		return m
	}
	return "video/mp4"
}

func VirtualPath(name string) string {
	return fmt.Sprintf("%s/%s", MoviesPath, name)
}
