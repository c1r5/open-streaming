package engine

import (
	"io"
	"log"
	"os"
	"sync"

	libtorrent "github.com/anacrolix/torrent"
)

// hybridReadSeeker reads from os.File for complete pieces, torrent reader for the rest.
type hybridReadSeeker struct {
	local  *os.File
	remote io.ReadSeeker
	source *libtorrent.File
	size   int64
	pos    int64
	mu     sync.Mutex
}

func (h *hybridReadSeeker) Read(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pos >= h.size {
		return 0, io.EOF
	}

	if h.pieceCompleteAt(h.pos) {
		log.Printf("hybrid: read LOCAL pos=%d len=%d", h.pos, len(p))
		n, err := h.local.ReadAt(p, h.pos)
		h.pos += int64(n)
		h.remote.Seek(h.pos, io.SeekStart)
		return n, err
	}

	log.Printf("hybrid: read REMOTE pos=%d len=%d", h.pos, len(p))
	h.remote.Seek(h.pos, io.SeekStart)
	n, err := h.remote.Read(p)
	h.pos += int64(n)
	return n, err
}

func (h *hybridReadSeeker) Seek(offset int64, whence int) (int64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = h.pos + offset
	case io.SeekEnd:
		newPos = h.size + offset
	}

	log.Printf("hybrid: seek whence=%d offset=%d -> pos=%d", whence, offset, newPos)
	h.pos = newPos
	return newPos, nil
}

func (h *hybridReadSeeker) Close() error {
	log.Printf("hybrid: close (final pos=%d, size=%d)", h.pos, h.size)
	h.local.Close()
	if closer, ok := h.remote.(io.Closer); ok {
		closer.Close()
	}
	return nil
}

func (h *hybridReadSeeker) pieceCompleteAt(pos int64) bool {
	states := h.source.State()
	var offset int64
	for _, s := range states {
		if pos >= offset && pos < offset+s.Bytes {
			return s.Ok && s.Complete
		}
		offset += s.Bytes
	}
	return false
}
