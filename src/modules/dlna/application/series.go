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
)

type placedVideo struct {
	row  torrentRow
	full string
	dir  string
	base string
}

func (s *Service) loadLibrary() (movies []torrentRow, series []placedVideo, err error) {
	var rows []torrentRow
	if err = s.db.Where("deleted_at IS NULL").Find(&rows).Error; err != nil {
		return nil, nil, fmt.Errorf("dlna: query torrents: %w", err)
	}

	grouped := map[string][]torrentRow{}
	order := make([]string, 0)
	for _, row := range rows {
		if _, ok := grouped[row.Hash]; !ok {
			order = append(order, row.Hash)
		}
		grouped[row.Hash] = append(grouped[row.Hash], row)
	}

	for _, hash := range order {
		group := grouped[hash]
		if !isSeriesGroup(group) {
			movies = append(movies, group[0])
			continue
		}
		for _, row := range group {
			full := torrentdomain.LibraryPath(row.TorrentName)
			series = append(series, placedVideo{
				row:  row,
				full: full,
				dir:  path.Dir(full),
				base: path.Base(full),
			})
		}
	}
	return movies, series, nil
}

func isSeriesGroup(group []torrentRow) bool {
	if len(group) > 1 {
		return true
	}
	return len(group) == 1 && torrentdomain.HasEpisode(group[0].TorrentName)
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

func (s *Service) videoItem(id, parent, title, streamURL, mime string, row torrentRow) upnpav.Item {
	return upnpav.Item{
		Object: upnpav.Object{
			ID:         id,
			ParentID:   parent,
			Restricted: 1,
			Title:      title,
			Class:      "object.item.videoItem",
			Date:       upnpav.Timestamp{Time: row.CreatedAt},
		},
		Res: []upnpav.Resource{{
			URL:          streamURL,
			ProtocolInfo: fmt.Sprintf("http-get:*:%s:DLNA.ORG_OP=01;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=01700000000000000000000000000000", mime),
			Size:         uint64(row.Size),
		}},
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
		objs = append(objs, s.videoItem(
			itemObjectID(item),
			item.dir,
			item.base,
			fmt.Sprintf("%s/stream/watch/%d", s.baseURL, item.row.ID),
			domain.MIMEForPath(item.base),
			item.row,
		))
	}
	return objs
}
