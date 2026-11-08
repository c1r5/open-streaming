package libtorrent

import (
	"fmt"
	"path/filepath"

	libtorrent "github.com/anacrolix/torrent"
)

func (t *TorrentEngine) LocalFile(hash string, index int) (string, bool, error) {
	if !torrentHash(hash) {
		return "", false, fmt.Errorf("torrent engine: invalid hash")
	}
	tor := t.ensureLoaded(hash)
	if tor == nil || tor.Info() == nil {
		return "", false, fmt.Errorf("torrent engine: torrent files unknown")
	}
	files := tor.Files()
	if index < 0 || index >= len(files) {
		return "", false, fmt.Errorf("torrent engine: file index %d not in torrent", index)
	}
	file := files[index]
	complete := file.Length() > 0 && file.BytesCompleted() == file.Length()
	return filepath.Join(t.dataDir, file.Path()), complete, nil
}

func (t *TorrentEngine) DetachFile(hash string, index int) error {
	if !torrentHash(hash) {
		return fmt.Errorf("torrent engine: invalid hash")
	}
	tor := t.ensureLoaded(hash)
	if tor == nil || tor.Info() == nil {
		return fmt.Errorf("torrent engine: torrent files unknown")
	}
	files := tor.Files()
	if index < 0 || index >= len(files) {
		return fmt.Errorf("torrent engine: file index %d not in torrent", index)
	}
	files[index].SetPriority(libtorrent.PiecePriorityNone)
	if t.fileCache != nil {
		t.fileCache.Delete(hash)
		t.fileCache.Delete(fmt.Sprintf("%s:%d", hash, index))
	}
	return nil
}
