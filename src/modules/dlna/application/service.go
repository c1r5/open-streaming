package application

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/c1r5/open-streaming/src/modules/dlna/domain"
	"gorm.io/gorm"
)

type torrentRow struct {
	ID        uint
	Title     string
	Size      int64
	CreatedAt time.Time
}

type Service struct {
	db      *gorm.DB
	baseURL string // e.g. "http://192.168.1.10:3000"
}

func NewService(db *gorm.DB, baseURL string) *Service {
	return &Service{db: db, baseURL: baseURL}
}

func (s *Service) BrowseDirectChildren(path, rootObjectPath, host, userAgent string) ([]interface{}, error) {
	movies, series, err := s.loadLibrary()
	if err != nil {
		return nil, err
	}
	switch path {
	case domain.RootPath, "":
		return s.browseRoot(movies, series), nil
	case domain.MoviesPath:
		return s.browseMovies(movies), nil
	default:
		if findPlaced(path, series) != nil {
			return []interface{}{}, nil
		}
		if !seriesDir(path, series) {
			return nil, fmt.Errorf("unknown path: %s", path)
		}
		return s.seriesDirectChildren(path, series), nil
	}
}

func (s *Service) BrowseMetadata(path, rootObjectPath, host, userAgent string) (interface{}, error) {
	movies, series, err := s.loadLibrary()
	if err != nil {
		return nil, err
	}
	switch path {
	case domain.RootPath, "":
		count := 1
		if len(series) > 0 {
			count = 2
		}
		return s.folderContainer("0", "Root", "-1", count), nil
	case domain.MoviesPath:
		return s.folderContainer(domain.MoviesPath, "Movies", "0", len(movies)), nil
	default:
		if item := findPlaced(path, series); item != nil {
			return s.episodeItem(*item), nil
		}
		if !seriesDir(path, series) {
			return nil, fmt.Errorf("unknown path: %s", path)
		}
		folders, items := seriesChildren(path, series)
		title := path
		if path != domain.SeriesPath {
			title = path[strings.LastIndex(path, "/")+1:]
		} else {
			title = "Series"
		}
		return s.folderContainer(path, title, containerParent(path), len(folders)+len(items)), nil
	}
}

func (s *Service) browseRoot(movies []torrentRow, series []placedVideo) []interface{} {
	root := []interface{}{
		s.folderContainer(domain.MoviesPath, "Movies", "0", len(movies)),
	}
	if len(series) == 0 {
		return root
	}
	folders, items := seriesChildren(domain.SeriesPath, series)
	root = append(root, s.folderContainer(domain.SeriesPath, "Series", "0", len(folders)+len(items)))
	return root
}

func (s *Service) browseMovies(movies []torrentRow) []interface{} {
	objs := make([]interface{}, 0, len(movies))
	for _, row := range movies {
		objs = append(objs, s.videoItem(
			domain.VirtualPath(row.Title),
			domain.MoviesPath,
			row.Title,
			fmt.Sprintf("%s/movies/%d/watch", s.baseURL, row.ID),
			domain.MIMEForPath(row.Title),
			"",
			"",
			row,
		))
	}
	return objs
}

func StreamBaseURL(host string, port string) string {
	u := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%s", host, port),
	}
	return u.String()
}
