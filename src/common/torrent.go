package common

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
)

type AddTorrentOptions struct {
	Hash     string   `json:"hash"`
	Trackers []string `json:"trackers,omitempty"`
	Persist  bool     `json:"persist"`
}

type ITorrentService interface {
	AddTorrent(options *AddTorrentOptions) (uint, error)
	DeleteTorrent(hash string) error
	SearchTorrents(query string) ([]*TorrentSearchResult, error)
}

type ITorrentController interface {
	SearchHandler() HandlerFunc
	AddHandler() HandlerFunc
	DeleteHandler() HandlerFunc
}

type MagnetModel struct {
	Name    string   `json:"name,omitempty"`
	Hash    string   `json:"infoHash"`
	Sources []string `json:"sources,omitempty"`
}

func (t MagnetModel) Trackers() []string {
	trackers := make([]string, 0, len(t.Sources))
	for _, s := range t.Sources {
		if after, ok := strings.CutPrefix(s, "tracker:"); ok {
			trackers = append(trackers, after)
		}
	}
	return trackers
}

func (t MagnetModel) Magnet() string {
	trackers := t.Trackers()
	params := url.Values{"xt": {"urn:btih:" + t.Hash}}
	for _, tr := range trackers {
		params.Add("tr", tr)
	}
	return "magnet:?" + params.Encode()
}

func Parse(magnetURI string) (*MagnetModel, error) {
	info := &MagnetModel{}

	magnetV1, err := metainfo.ParseMagnetUri(magnetURI)
	if err != nil {
		magnetV2, err2 := metainfo.ParseMagnetV2Uri(magnetURI)
		if err2 != nil {
			return nil, fmt.Errorf("magnet: parse failed: %w", err)
		}
		if magnetV2.InfoHash.Value.HexString() != "" {
			info.Hash = magnetV2.InfoHash.Value.HexString()
			info.Name = magnetV2.DisplayName
			for _, tr := range magnetV2.Trackers {
				info.Sources = append(info.Sources, "tracker:"+tr)
			}
			return info, nil
		}
		return nil, fmt.Errorf("magnet: no valid info hash found")
	}

	if magnetV1.InfoHash.HexString() != "" {
		info.Hash = magnetV1.InfoHash.HexString()
		info.Name = magnetV1.DisplayName
		for _, tr := range magnetV1.Trackers {
			info.Sources = append(info.Sources, "tracker:"+tr)
		}
		return info, nil
	}

	return nil, fmt.Errorf("magnet: no valid info hash found")
}
