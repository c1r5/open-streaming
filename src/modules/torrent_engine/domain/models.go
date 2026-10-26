package domain

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/anacrolix/torrent/metainfo"
)

type AddTorrentOptions struct {
	Hash   string `json:"hash"`
	Magnet string `json:"magnet"`
}

type TorrentFile struct {
	Path   string
	Index  int
	Name   string
	Size   int64
	Reader io.ReadSeeker
}

type ITorrentEngine interface {
	Resolve(hash string) (*TorrentFile, error)
	ResolveMagnet(magnet *MagnetModel) (*TorrentFile, error)
	ResolveFile(hash string, index int) (*TorrentFile, error)
	ListVideos(magnet *MagnetModel) ([]VideoMeta, error)
	PersistTorrentFile(hash string) error
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
	magnetURI = normalizeBase32Infohash(magnetURI)

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

// normalizeBase32Infohash uppercases 32-character v1 infohashes.
// encoding/base32 rejects lowercase, and magnet links often use it.
func normalizeBase32Infohash(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return uri
	}

	q := u.Query()
	xts := q["xt"]
	changed := false
	for i, xt := range xts {
		encoded, ok := strings.CutPrefix(xt, "urn:btih:")
		if !ok || len(encoded) != 32 {
			continue
		}
		upper := strings.ToUpper(encoded)
		if upper == encoded {
			continue
		}
		xts[i] = "urn:btih:" + upper
		changed = true
	}
	if !changed {
		return uri
	}

	q["xt"] = xts
	u.RawQuery = q.Encode()
	return u.String()
}
