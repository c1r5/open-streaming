package libtorrent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

func TestRemoveDeletesContentInsideDataDir(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "outside-"+filepath.Base(dir)+".txt")
	if err := os.WriteFile(outside, []byte("stay"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(outside) })

	show := filepath.Join(dir, "Show")
	if err := os.MkdirAll(show, 0o755); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(show, "e01.mkv")
	if err := os.WriteFile(video, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(video+".part", []byte("part"), 0o644); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(keep, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	const hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	info := metainfo.Info{
		Name:        "Show",
		PieceLength: 16 * 1024,
		Files: []metainfo.FileInfo{
			{Path: []string{"e01.mkv"}, Length: 5},
			{Path: []string{"..", filepath.Base(outside)}, Length: 4},
		},
	}
	payload, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	meta := metainfo.MetaInfo{InfoBytes: payload}
	sidecar := filepath.Join(dir, hash+".torrent")
	file, err := os.Create(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	if err := meta.Write(file); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	engine, err := NewEngine(nil, nil, EngineOptions{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Remove(hash); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(video); !os.IsNotExist(err) {
		t.Fatalf("video stat %v", err)
	}
	if _, err := os.Stat(video + ".part"); !os.IsNotExist(err) {
		t.Fatalf("part stat %v", err)
	}
	if _, err := os.Stat(show); !os.IsNotExist(err) {
		t.Fatalf("show dir stat %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("keep file %v", err)
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("sidecar removed early %v", err)
	}

	if err := engine.Remove("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"); err == nil {
		t.Fatal("missing torrent returned nil")
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("sidecar lost after unknown hash %v", err)
	}

	if err := engine.RemoveTorrentFile(hash); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sidecar); !os.IsNotExist(err) {
		t.Fatalf("sidecar stat %v", err)
	}
}

func TestRemoveFileDeletesOnlyThatFile(t *testing.T) {
	dir := t.TempDir()
	show := filepath.Join(dir, "Show")
	if err := os.MkdirAll(show, 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(show, "e01.mkv")
	second := filepath.Join(show, "e02.mkv")
	subtitle := filepath.Join(show, "e01.srt")
	for _, path := range []string{first, second, subtitle, first + ".part"} {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	info := metainfo.Info{
		Name:        "Show",
		PieceLength: 16 * 1024,
		Files: []metainfo.FileInfo{
			{Path: []string{"e01.mkv"}, Length: 1},
			{Path: []string{"e01.srt"}, Length: 1},
			{Path: []string{"e02.mkv"}, Length: 1},
		},
	}
	payload, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	meta := metainfo.MetaInfo{InfoBytes: payload}
	sidecar := filepath.Join(dir, hash+".torrent")
	file, err := os.Create(sidecar)
	if err != nil {
		t.Fatal(err)
	}
	if err := meta.Write(file); err != nil {
		file.Close()
		t.Fatal(err)
	}
	file.Close()

	engine, err := NewEngine(nil, nil, EngineOptions{DataDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.RemoveFile(hash, 0); err != nil {
		t.Fatal(err)
	}
	if err := engine.RemoveFile(hash, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Fatalf("e01 stat %v", err)
	}
	if _, err := os.Stat(first + ".part"); !os.IsNotExist(err) {
		t.Fatalf("e01 part stat %v", err)
	}
	if _, err := os.Stat(subtitle); !os.IsNotExist(err) {
		t.Fatalf("subtitle stat %v", err)
	}
	if _, err := os.Stat(second); err != nil {
		t.Fatalf("e02 %v", err)
	}
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("sidecar %v", err)
	}
	if err := engine.RemoveFile(hash, 9); err == nil {
		t.Fatal("missing index returned nil")
	}
}
