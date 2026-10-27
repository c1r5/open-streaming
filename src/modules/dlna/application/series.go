package application

import (
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/anacrolix/dms/upnpav"
	"github.com/c1r5/open-streaming/src/modules/dlna/domain"
	torrentdomain "github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
)

type placedVideo struct {
	row         torrentRow
	seriesID    uint
	full        string
	dir         string
	base        string
	subtitleExt string
}

func (s *Service) loadLibrary() (movies []torrentRow, series []placedVideo, err error) {
	var movieRows []persistence.MovieModel
	if err = s.db.Preload("Torrent").Find(&movieRows).Error; err != nil {
		return nil, nil, fmt.Errorf("dlna: query movies: %w", err)
	}
	for _, movie := range movieRows {
		movies = append(movies, torrentRow{
			ID:        movie.ID,
			Title:     movie.Torrent.Name,
			Size:      movie.Size,
			CreatedAt: movie.Torrent.CreatedAt,
		})
	}

	var episodes []persistence.EpisodeModel
	if err = s.db.Preload("Series.Torrent").Find(&episodes).Error; err != nil {
		return nil, nil, fmt.Errorf("dlna: query episodes: %w", err)
	}
	for _, episode := range episodes {
		full := torrentdomain.LibraryPath(episode.Series.Torrent.Name + "/" + episode.Name)
		series = append(series, placedVideo{
			row: torrentRow{
				ID:        episode.ID,
				Title:     episode.Name,
				Size:      episode.Size,
				CreatedAt: episode.CreatedAt,
			},
			seriesID:    episode.SeriesID,
			full:        full,
			dir:         path.Dir(full),
			base:        path.Base(full),
			subtitleExt: episode.SubtitleExt,
		})
	}
	return movies, series, nil
}

func seriesChildren(parent string, series []placedVideo) (folders []string, items []placedVideo) {
	prefix := parent + "/"
	seen := map[string]bool{}
	for _, item := range series {
		if !strings.HasPrefix(item.full, prefix) {
			continue
		}
		rest := strings.TrimPrefix(item.full, prefix)
		parts := strings.Split(rest, "/")
		if len(parts) == 1 {
			items = append(items, item)
			continue
		}
		if !seen[parts[0]] {
			seen[parts[0]] = true
			folders = append(folders, parts[0])
		}
	}
	sort.Strings(folders)
	sort.SliceStable(items, func(i, j int) bool {
		return torrentdomain.LessEpisodeName(items[i].base, items[j].base)
	})
	return folders, items
}

func seriesDir(path string, series []placedVideo) bool {
	if len(series) == 0 {
		return false
	}
	if path == domain.SeriesPath {
		return true
	}
	prefix := path + "/"
	for _, item := range series {
		if strings.HasPrefix(item.full, prefix) {
			return true
		}
	}
	return false
}

func findPlaced(objectID string, series []placedVideo) *placedVideo {
	for i := range series {
		if itemObjectID(series[i]) == objectID {
			return &series[i]
		}
	}
	return nil
}

func itemObjectID(item placedVideo) string {
	return item.full + "#" + strconv.FormatUint(uint64(item.row.ID), 10)
}

func (s *Service) folderContainer(id, title, parent string, childCount int) upnpav.Container {
	return upnpav.Container{
		Object: upnpav.Object{
			ID:         id,
			ParentID:   parent,
			Restricted: 1,
			Title:      title,
			Class:      "object.container.storageFolder",
		},
		ChildCount: childCount,
	}
}

func (s *Service) episodeItem(item placedVideo) upnpav.Item {
	watchURL := fmt.Sprintf("%s/series/%d/episodes/%d/watch", s.baseURL, item.seriesID, item.row.ID)
	subtitleURL := ""
	subtitleMIME := ""
	if item.subtitleExt != "" {
		subtitleURL = fmt.Sprintf("%s/series/%d/episodes/%d/subtitle", s.baseURL, item.seriesID, item.row.ID)
		subtitleMIME = domain.SubtitleMIME(item.subtitleExt)
	}
	return s.videoItem(itemObjectID(item), item.dir, item.base, watchURL, domain.MIMEForPath(item.base), subtitleURL, subtitleMIME, item.row)
}

func containerParent(id string) string {
	if id == domain.MoviesPath || id == domain.SeriesPath {
		return "0"
	}
	parent := path.Dir(id)
	if parent == "/" || parent == "." {
		return "0"
	}
	return parent
}

func (s *Service) videoItem(id, parent, title, streamURL, mime, subtitleURL, subtitleMIME string, row torrentRow) upnpav.Item {
	resources := []upnpav.Resource{{
		URL:          streamURL,
		ProtocolInfo: fmt.Sprintf("http-get:*:%s:DLNA.ORG_OP=01;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=01700000000000000000000000000000", mime),
		Size:         uint64(row.Size),
	}}
	if subtitleURL != "" {
		resources = append(resources, upnpav.Resource{
			URL:          subtitleURL,
			ProtocolInfo: fmt.Sprintf("http-get:*:%s:*", subtitleMIME),
		})
	}
	return upnpav.Item{
		Object: upnpav.Object{
			ID:         id,
			ParentID:   parent,
			Restricted: 1,
			Title:      title,
			Class:      "object.item.videoItem",
			Date:       upnpav.Timestamp{Time: row.CreatedAt},
		},
		Res: resources,
	}
}

func (s *Service) seriesDirectChildren(parent string, series []placedVideo) []interface{} {
	folders, items := seriesChildren(parent, series)
	objs := make([]interface{}, 0, len(folders)+len(items))
	for _, folder := range folders {
		id := parent + "/" + folder
		belowFolders, belowItems := seriesChildren(id, series)
		objs = append(objs, s.folderContainer(id, folder, parent, len(belowFolders)+len(belowItems)))
	}
	for _, item := range items {
		objs = append(objs, s.episodeItem(item))
	}
	return objs
}
