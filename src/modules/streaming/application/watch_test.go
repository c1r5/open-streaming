package application

import (
	"fmt"
	"testing"

	enginedomain "github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestGetStreamFile_usesIndexOnlyForPack(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}); err != nil {
		t.Fatal(err)
	}

	bolt := persistence.TorrentModel{Hash: "bolt", FileIndex: 0, TorrentName: "bolt_supercao.mkv", Size: 10}
	episodeOne := persistence.TorrentModel{Hash: "pack", FileIndex: 3, TorrentName: "S01E01.mkv", Size: 100}
	episodeTwo := persistence.TorrentModel{Hash: "pack", FileIndex: 7, TorrentName: "S01E02.mkv", Size: 500}
	for _, row := range []*persistence.TorrentModel{&bolt, &episodeOne, &episodeTwo} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	engine := &streamEngine{}
	service := NewService(engine, db)

	movie, err := service.GetStreamFile(bolt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if movie.Path != "largest" || engine.resolveCalls != 1 || len(engine.fileCalls) != 0 {
		t.Fatalf("movie path %q resolve %d files %#v", movie.Path, engine.resolveCalls, engine.fileCalls)
	}

	first, err := service.GetStreamFile(episodeOne.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.GetStreamFile(episodeTwo.ID)
	if err != nil {
		t.Fatal(err)
	}
	if first.Path != "3" || second.Path != "7" || engine.resolveCalls != 1 {
		t.Fatalf("episodes %q %q resolve %d calls %#v", first.Path, second.Path, engine.resolveCalls, engine.fileCalls)
	}
}

type streamEngine struct {
	resolveCalls int
	fileCalls    []int
}

func (e *streamEngine) Resolve(string) (*enginedomain.TorrentFile, error) {
	e.resolveCalls++
	return &enginedomain.TorrentFile{Path: "largest", Name: "largest"}, nil
}

func (e *streamEngine) ResolveMagnet(*enginedomain.MagnetModel) (*enginedomain.TorrentFile, error) {
	return e.Resolve("")
}

func (e *streamEngine) ResolveFile(_ string, index int) (*enginedomain.TorrentFile, error) {
	e.fileCalls = append(e.fileCalls, index)
	name := fmt.Sprintf("%d", index)
	return &enginedomain.TorrentFile{Path: name, Name: name}, nil
}

func (e *streamEngine) ListVideos(*enginedomain.MagnetModel) ([]enginedomain.VideoMeta, error) {
	return nil, nil
}

func (e *streamEngine) PersistTorrentFile(string) error { return nil }
