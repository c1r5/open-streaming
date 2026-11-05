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
	ListFiles(magnet *MagnetModel) ([]VideoMeta, error)
	PersistTorrentFile(hash string) error
	// Remove stops the swarm and deletes the torrent's content files.
	// The <hash>.torrent sidecar stays until RemoveTorrentFile.
	Remove(hash string) error
	RemoveTorrentFile(hash string) error
	// RemoveFile stops one file and deletes it. The swarm stays.
	RemoveFile(hash string, index int) error
	// IgnoreFiles sets priority none on those file indexes.
	IgnoreFiles(hash string, indexes []int) error
}

type MagnetModel struct {
	Name    string   `json:"name,omitempty"`
	Hash    string   `json:"infoHash"`
	Sources []string `json:"sources,omitempty"`
}

func (t MagnetModel) Trackers() []string {
	trackers := make([]string, 0, len(t.Sources))
	seen := make(map[string]struct{}, len(t.Sources))
	for _, s := range t.Sources {
		tr, ok := SupportedTracker(s)
		if !ok {
			continue
		}
		if _, dup := seen[tr]; dup {
			continue
		}
		seen[tr] = struct{}{}
		trackers = append(trackers, tr)
	}
	return trackers
}

// SupportedTracker returns a tracker URL anacrolix can announce to.
// Torrentio sources and some magnets prefix the URL with "tracker:".
// Any other scheme reaches tracker.NewClient and panics the process with "unknown scheme".
func SupportedTracker(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw[0] == '*' {
		return "", false
	}
	for {
		after, ok := strings.CutPrefix(raw, "tracker:")
		if !ok {
			break
		}
		raw = strings.TrimSpace(after)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "udp", "udp4", "udp6", "ws", "wss":
		return raw, true
	default:
		return "", false
	}
}

func trackerSource(raw string) (string, bool) {
	clean, ok := SupportedTracker(raw)
	if !ok {
		return "", false
	}
	return "tracker:" + clean, true
}

// SanitizeAnnounce drops announce URLs that would panic the torrent client.
func SanitizeAnnounce(mi *metainfo.MetaInfo) {
	if mi == nil {
		return
	}
	if tr, ok := SupportedTracker(mi.Announce); ok {
		mi.Announce = tr
	} else {
		mi.Announce = ""
	}
	filtered := make(metainfo.AnnounceList, 0, len(mi.AnnounceList))
	for _, tier := range mi.AnnounceList {
		kept := make([]string, 0, len(tier))
		for _, raw := range tier {
			if tr, ok := SupportedTracker(raw); ok {
				kept = append(kept, tr)
			}
		}
		if len(kept) > 0 {
			filtered = append(filtered, kept)
		}
	}
	mi.AnnounceList = filtered
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
				if source, ok := trackerSource(tr); ok {
					info.Sources = append(info.Sources, source)
				}
			}
			return info, nil
		}
		return nil, fmt.Errorf("magnet: no valid info hash found")
	}

	if magnetV1.InfoHash.HexString() != "" {
		info.Hash = magnetV1.InfoHash.HexString()
		info.Name = magnetV1.DisplayName
		for _, tr := range magnetV1.Trackers {
			if source, ok := trackerSource(tr); ok {
				info.Sources = append(info.Sources, source)
			}
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
