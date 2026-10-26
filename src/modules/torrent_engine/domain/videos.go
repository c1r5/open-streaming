package domain

import (
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// VideoMeta is one video inside a torrent. Index is the position in the
// torrent file list. Name is the display path, with '/' between segments.
type VideoMeta struct {
	Index int
	Name  string
	Path  string
	Size  int64
}

var episodePattern = regexp.MustCompile(`(?i)S(\d{1,2})E(\d{1,2})`)

func IsSamplePath(filePath string) bool {
	normalized := strings.ReplaceAll(filePath, "\\", "/")
	for _, part := range strings.Split(normalized, "/") {
		if strings.EqualFold(part, "sample") {
			return true
		}
	}
	return false
}

func VideosForLibrary(files []VideoMeta) []VideoMeta {
	kept := make([]VideoMeta, 0, len(files))
	for _, file := range files {
		if IsSamplePath(file.Name) || IsSamplePath(file.Path) {
			continue
		}
		kept = append(kept, file)
	}
	return kept
}

func IsSeries(files []VideoMeta) bool {
	if len(files) > 1 {
		return true
	}
	return len(files) == 1 && episodePattern.MatchString(files[0].Name)
}

type episodeKey struct {
	ok     bool
	season int
	number int
}

func episodeOf(name string) episodeKey {
	match := episodePattern.FindStringSubmatch(path.Base(strings.ReplaceAll(name, "\\", "/")))
	if match == nil {
		return episodeKey{}
	}
	season, _ := strconv.Atoi(match[1])
	number, _ := strconv.Atoi(match[2])
	return episodeKey{ok: true, season: season, number: number}
}

func SortVideos(files []VideoMeta) {
	sort.SliceStable(files, func(i, j int) bool {
		left := episodeOf(files[i].Name)
		right := episodeOf(files[j].Name)
		if left.ok && right.ok {
			if left.season != right.season {
				return left.season < right.season
			}
			if left.number != right.number {
				return left.number < right.number
			}
		}
		if left.ok != right.ok {
			return left.ok
		}
		return files[i].Name < files[j].Name
	})
}

// PackTitle is the season directory shown for a series. A file that already
// sits at the torrent root, or directly under Series, keeps its own name.
func PackTitle(displayPath string) string {
	normalized := strings.Trim(strings.ReplaceAll(displayPath, "\\", "/"), "/")
	dir, base := path.Split(normalized)
	dir = strings.Trim(dir, "/")
	if dir == "" || strings.EqualFold(dir, "Series") {
		return base
	}
	return path.Base(dir)
}
