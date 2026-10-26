package application

import (
	"testing"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestAddTorrent_storesEachVideoAndKeepsOneListEntry(t *testing.T) {
	db := openTestDB(t)
	engine := &fakeEngine{}
	service := NewService(&ServiceOptions{TorrentEngine: engine, DB: db})

	const packHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	engine.videos = []domain.VideoMeta{
		{Index: 4, Name: "Series/Sample/The.Big.Bang.Theory.S01E00.sample.mkv", Size: 20},
		{
			Index: 7,
			Name:  "Series/Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV/The.Big.Bang.Theory.S01E02.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.mkv",
			Path:  "/data/e02.mkv",
			Size:  500,
		},
		{
			Index: 3,
			Name:  "Series/Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV/The.Big.Bang.Theory.S01E01.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.mkv",
			Path:  "/data/e01.mkv",
			Size:  100,
		},
	}

	firstID, err := service.AddTorrent(&domain.AddTorrentOptions{
		Magnet: "magnet:?xt=urn:btih:" + packHash,
	})
	if err != nil {
		t.Fatal(err)
	}

	var rows []persistence.TorrentModel
	if err := db.Where("hash = ?", packHash).Order("file_index").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows %d, want 2 (sample excluded)", len(rows))
	}
	if rows[0].FileIndex != 3 || rows[0].Size != 100 || rows[0].OutputPath != "/data/e01.mkv" {
		t.Fatalf("first stored file %+v", rows[0])
	}
	if rows[1].FileIndex != 7 || rows[1].Size != 500 {
		t.Fatalf("second stored file %+v", rows[1])
	}
	if rows[0].ID != firstID {
		t.Fatalf("add id %d, want first episode %d", firstID, rows[0].ID)
	}
	if len(engine.persisted) != 1 || engine.persisted[0] != packHash {
		t.Fatalf("persisted %#v", engine.persisted)
	}

	again, err := service.AddTorrent(&domain.AddTorrentOptions{
		Magnet: "magnet:?xt=urn:btih:" + packHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if again != firstID {
		t.Fatalf("second add id %d, want %d", again, firstID)
	}
	var count int64
	if err := db.Model(&persistence.TorrentModel{}).Where("hash = ?", packHash).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("count after second add %d", count)
	}

	listed, err := service.ListTorrents()
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Size != 600 || listed[0].ID != rows[0].ID || len(listed[0].Episodes) != 2 {
		t.Fatalf("list %+v, want one pack with both episodes", listed)
	}
	if listed[0].Name != "Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV" {
		t.Fatalf("pack title %q", listed[0].Name)
	}

	const movieHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	legacy := persistence.TorrentModel{Hash: movieHash, FileIndex: 0, TorrentName: "old.mkv", Size: 1}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	engine.videos = []domain.VideoMeta{{
		Index: 4,
		Name:  "bolt_supercao.mkv",
		Path:  "/data/bolt_supercao.mkv",
		Size:  539402598,
	}}
	movieID, err := service.AddTorrent(&domain.AddTorrentOptions{
		Magnet: "magnet:?xt=urn:btih:" + movieHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if movieID != legacy.ID {
		t.Fatalf("movie id %d, want legacy row %d", movieID, legacy.ID)
	}
	var movie persistence.TorrentModel
	if err := db.First(&movie, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if movie.FileIndex != 4 || movie.TorrentName != "bolt_supercao.mkv" || movie.Size != 539402598 {
		t.Fatalf("updated movie %+v", movie)
	}
	if err := db.Model(&persistence.TorrentModel{}).Where("hash = ?", movieHash).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("movie rows %d", count)
	}
}

type fakeEngine struct {
	videos    []domain.VideoMeta
	persisted []string
}

func (f *fakeEngine) Resolve(string) (*domain.TorrentFile, error) {
	return &domain.TorrentFile{}, nil
}

func (f *fakeEngine) ResolveMagnet(*domain.MagnetModel) (*domain.TorrentFile, error) {
	return &domain.TorrentFile{}, nil
}

func (f *fakeEngine) ResolveFile(string, int) (*domain.TorrentFile, error) {
	return &domain.TorrentFile{}, nil
}

func (f *fakeEngine) ListVideos(*domain.MagnetModel) ([]domain.VideoMeta, error) {
	return f.videos, nil
}

func (f *fakeEngine) PersistTorrentFile(hash string) error {
	f.persisted = append(f.persisted, hash)
	return nil
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}); err != nil {
		t.Fatal(err)
	}
	return db
}
