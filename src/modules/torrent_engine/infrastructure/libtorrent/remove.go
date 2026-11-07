package libtorrent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	libtorrent "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
)

func (t *TorrentEngine) Remove(hash string) error {
	rels, err := t.contentRels(hash)
	if err != nil {
		return err
	}
	t.drop(hash)
	for _, rel := range rels {
		if err := t.removeContent(rel); err != nil {
			return err
		}
	}
	return nil
}

func (t *TorrentEngine) RemoveFile(hash string, index int) error {
	rels, err := t.contentRels(hash)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(rels) {
		return fmt.Errorf("engine: remove: file index %d not in torrent", index)
	}
	if tor := t.ensureLoaded(hash); tor != nil {
		files := tor.Files()
		if index < len(files) {
			files[index].SetPriority(libtorrent.PiecePriorityNone)
		}
	}
	if t.fileCache != nil {
		t.fileCache.Delete(hash)
		t.fileCache.Delete(fmt.Sprintf("%s:%d", hash, index))
	}
	return t.removeContent(rels[index])
}

func (t *TorrentEngine) IgnoreFiles(hash string, indexes []int) error {
	if !torrentHash(hash) {
		return fmt.Errorf("engine: remove: invalid hash")
	}
	tor := t.ensureLoaded(hash)
	if tor == nil {
		return fmt.Errorf("engine: remove: torrent files unknown")
	}
	files := tor.Files()
	for _, index := range indexes {
		if index < 0 || index >= len(files) {
			return fmt.Errorf("engine: remove: file index %d not in torrent", index)
		}
		files[index].SetPriority(libtorrent.PiecePriorityNone)
	}
	return nil
}

func (t *TorrentEngine) ensureLoaded(hash string) *libtorrent.Torrent {
	if tor := t.loaded(hash); tor != nil {
		return tor
	}
	if t.client == nil {
		return nil
	}
	path := filepath.Join(t.dataDir, hash+".torrent")
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	tor, err := t.client.AddTorrentFromFile(path)
	if err != nil {
		return nil
	}
	return tor
}

func (t *TorrentEngine) RemoveTorrentFile(hash string) error {
	if !torrentHash(hash) {
		return fmt.Errorf("engine: remove: invalid hash")
	}
	base, err := t.dataRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(base, hash+".torrent")
	if !insideDir(base, path) {
		return fmt.Errorf("engine: remove: sidecar escapes data dir")
	}
	return removeIfPresent(path)
}

func (t *TorrentEngine) contentRels(hash string) ([]string, error) {
	if !torrentHash(hash) {
		return nil, fmt.Errorf("engine: remove: invalid hash")
	}
	if tor := t.loaded(hash); tor != nil && tor.Info() != nil {
		rels := make([]string, 0, len(tor.Files()))
		for _, file := range tor.Files() {
			rels = append(rels, file.Path())
		}
		if len(rels) > 0 {
			return rels, nil
		}
	}

	torrentPath := filepath.Join(t.dataDir, hash+".torrent")
	file, err := os.Open(torrentPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("engine: remove: torrent files unknown")
		}
		return nil, err
	}
	defer file.Close()

	meta, err := metainfo.Load(file)
	if err != nil {
		return nil, fmt.Errorf("engine: remove: read torrent: %w", err)
	}
	info, err := meta.UnmarshalInfo()
	if err != nil {
		return nil, fmt.Errorf("engine: remove: read info: %w", err)
	}
	rels := make([]string, 0)
	for _, item := range info.UpvertedFiles() {
		parts := make([]string, 0, 1+len(item.BestPath()))
		if info.BestName() != metainfo.NoName {
			parts = append(parts, info.BestName())
		}
		parts = append(parts, item.BestPath()...)
		rels = append(rels, strings.Join(parts, "/"))
	}
	if len(rels) == 0 {
		return nil, fmt.Errorf("engine: remove: torrent files unknown")
	}
	return rels, nil
}

func (t *TorrentEngine) drop(hash string) {
	if tor := t.loaded(hash); tor != nil {
		tor.Drop()
	}
	if t.cache != nil {
		t.cache.DeletePrefix(hash)
	}
	if t.fileCache != nil {
		t.fileCache.DeletePrefix(hash)
	}
}

func (t *TorrentEngine) Swarm(hash string) (domain.SwarmView, bool) {
	tor := t.loaded(hash)
	if tor == nil {
		return domain.SwarmView{}, false
	}
	view := domain.SwarmView{Peers: tor.Stats().ActivePeers}
	if info := tor.Info(); info != nil {
		view.Name = info.Name
	}
	return view, true
}

func (t *TorrentEngine) loaded(hash string) *libtorrent.Torrent {
	if t.client == nil {
		return nil
	}
	for _, tor := range t.client.Torrents() {
		if strings.EqualFold(tor.InfoHash().HexString(), hash) {
			return tor
		}
	}
	return nil
}

func (t *TorrentEngine) removeContent(rel string) error {
	parts, ok := safeRel(rel)
	if !ok {
		return nil
	}
	base, err := t.dataRoot()
	if err != nil {
		return err
	}
	path := filepath.Join(append([]string{base}, parts...)...)
	if !insideDir(base, path) {
		return nil
	}
	if err := removeIfPresent(path); err != nil {
		return err
	}
	if err := removeIfPresent(path + ".part"); err != nil {
		return err
	}
	removeEmptyDirs(filepath.Dir(path), base)
	return nil
}

func (t *TorrentEngine) dataRoot() (string, error) {
	dir := t.dataDir
	if dir == "" {
		dir = "."
	}
	return filepath.Abs(dir)
}

func torrentHash(hash string) bool {
	if len(hash) != 40 {
		return false
	}
	for _, r := range hash {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func safeRel(rel string) ([]string, bool) {
	rel = filepath.ToSlash(rel)
	if rel == "" || strings.HasPrefix(rel, "/") {
		return nil, false
	}
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, false
		}
	}
	return parts, true
}

func insideDir(base, path string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func removeIfPresent(path string) error {
	err := os.Remove(path)
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

func removeEmptyDirs(dir, base string) {
	for dir != base {
		rel, err := filepath.Rel(base, dir)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
