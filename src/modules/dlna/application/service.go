package application

import (
	"fmt"
	"net/url"
	"time"

	"github.com/anacrolix/dms/upnpav"
	"github.com/c1r5/open-streaming/src/modules/dlna/domain"
	"gorm.io/gorm"
)

type torrentRow struct {
	ID          uint
	TorrentName string
	Hash        string
	Size        int64
	CreatedAt   time.Time
}

func (torrentRow) TableName() string { return "torrents" }

type Service struct {
	db      *gorm.DB
	baseURL string // e.g. "http://192.168.1.10:3000"
}

func NewService(db *gorm.DB, baseURL string) *Service {
	return &Service{db: db, baseURL: baseURL}
}

func (s *Service) BrowseDirectChildren(path, rootObjectPath, host, userAgent string) ([]interface{}, error) {
	switch path {
	case domain.RootPath, "":
		return s.browseRoot()
	case domain.MoviesPath:
		return s.browseMovies(host)
	default:
		return nil, fmt.Errorf("unknown path: %s", path)
	}
}

func (s *Service) BrowseMetadata(path, rootObjectPath, host, userAgent string) (interface{}, error) {
	switch path {
	case domain.RootPath, "":
		return upnpav.Container{
			Object: upnpav.Object{
				ID:         "0",
				ParentID:   "-1",
				Restricted: 1,
				Title:      "Root",
				Class:      "object.container.storageFolder",
			},
			ChildCount: 1,
		}, nil
	case domain.MoviesPath:
		count, _ := s.torrentCount()
		return upnpav.Container{
			Object: upnpav.Object{
				ID:         domain.MoviesPath,
				ParentID:   "0",
				Restricted: 1,
				Title:      "Movies",
				Class:      "object.container.storageFolder",
			},
			ChildCount: count,
		}, nil
	default:
		return nil, fmt.Errorf("unknown path: %s", path)
	}
}

func (s *Service) browseRoot() ([]interface{}, error) {
	count, _ := s.torrentCount()
	return []interface{}{
		upnpav.Container{
			Object: upnpav.Object{
				ID:         domain.MoviesPath,
				ParentID:   "0",
				Restricted: 1,
				Title:      "Movies",
				Class:      "object.container.storageFolder",
			},
			ChildCount: count,
		},
	}, nil
}

func (s *Service) browseMovies(host string) ([]interface{}, error) {
	var rows []torrentRow
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("dlna: query torrents: %w", err)
	}

	objs := make([]interface{}, 0, len(rows))
	for _, r := range rows {
		streamURL := fmt.Sprintf("%s/stream/watch/%d", s.baseURL, r.ID)
		mimeType := domain.MIMEForPath(r.TorrentName)

		item := upnpav.Item{
			Object: upnpav.Object{
				ID:         domain.VirtualPath(r.TorrentName),
				ParentID:   domain.MoviesPath,
				Restricted: 1,
				Title:      r.TorrentName,
				Class:      "object.item.videoItem",
				Date:       upnpav.Timestamp{Time: r.CreatedAt},
			},
			Res: []upnpav.Resource{
				{
					URL:          streamURL,
					ProtocolInfo: fmt.Sprintf("http-get:*:%s:DLNA.ORG_OP=01;DLNA.ORG_CI=0;DLNA.ORG_FLAGS=01700000000000000000000000000000", mimeType),
					Size:         uint64(r.Size),
				},
			},
		}
		objs = append(objs, item)
	}
	return objs, nil
}

func (s *Service) torrentCount() (int, error) {
	var count int64
	if err := s.db.Table("torrents").Where("deleted_at IS NULL").Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func StreamBaseURL(host string, port string) string {
	u := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%s", host, port),
	}
	return u.String()
}
