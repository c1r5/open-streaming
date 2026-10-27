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
	const episodeDir = "Series/Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV"
	engine.videos = []domain.VideoMeta{
		{Index: 4, Name: "Series/Sample/The.Big.Bang.Theory.S01E00.sample.mkv", Size: 20},
		{
			Index: 7,
			Name:  episodeDir + "/The.Big.Bang.Theory.S01E02.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.mkv",
			Path:  "/data/e02.mkv",
			Size:  500,
		},
		{
			Index: 3,
			Name:  episodeDir + "/The.Big.Bang.Theory.S01E01.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.mkv",
			Path:  "/data/e01.mkv",
			Size:  100,
		},
	}
	engine.files = append([]domain.VideoMeta{
		{Index: 0, Name: "other/The.Big.Bang.Theory.S01E01.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.srt"},
		{Index: 1, Name: episodeDir + "/The.Big.Bang.Theory.S01E01.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.vtt"},
		{Index: 9, Name: episodeDir + "/The.Big.Bang.Theory.S01E01.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.srt"},
		{Index: 2, Name: episodeDir + "/The.Big.Bang.Theory.S01E01.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.srt"},
	}, engine.videos...)

	kind, firstID, err := service.AddTorrent(&domain.AddTorrentOptions{
		Magnet: "magnet:?xt=urn:btih:" + packHash,
	})
	if err != nil {
		t.Fatal(err)
	}

	var torrent persistence.TorrentModel
	if err := db.Where("hash = ?", packHash).First(&torrent).Error; err != nil {
		t.Fatal(err)
	}
	const season = "Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV"
	if torrent.Name != season || torrent.Size != 600 {
		t.Fatalf("torrent %+v", torrent)
	}
	var series persistence.SeriesModel
	if err := db.Where("torrent_id = ?", torrent.ID).First(&series).Error; err != nil {
		t.Fatal(err)
	}
	var episodes []persistence.EpisodeModel
	if err := db.Where("series_id = ?", series.ID).Order("file_index").Find(&episodes).Error; err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 2 || episodes[0].FileIndex != 3 || episodes[0].Size != 100 || episodes[1].FileIndex != 7 || episodes[1].Size != 500 {
		t.Fatalf("episodes %+v", episodes)
	}
	if episodes[0].SubtitleIndex == nil || *episodes[0].SubtitleIndex != 2 || episodes[0].SubtitleExt != ".srt" {
		t.Fatalf("e01 subtitle %+v", episodes[0])
	}
	if episodes[1].SubtitleIndex != nil || episodes[1].SubtitleExt != "" {
		t.Fatalf("e02 subtitle %+v", episodes[1])
	}
	if kind != "series" || series.ID != firstID {
		t.Fatalf("add %s id %d, want series %d", kind, firstID, series.ID)
	}
	if len(engine.persisted) != 1 || engine.persisted[0] != packHash {
		t.Fatalf("persisted %#v", engine.persisted)
	}

	againKind, again, err := service.AddTorrent(&domain.AddTorrentOptions{
		Magnet: "magnet:?xt=urn:btih:" + packHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if againKind != "series" || again != firstID {
		t.Fatalf("second add %s id %d, want series %d", againKind, again, firstID)
	}
	var count int64
	if err := db.Model(&persistence.TorrentModel{}).Where("hash = ?", packHash).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("count after second add %d", count)
	}

	listed, err := service.ListSeries()
	if err != nil {
		t.Fatal(err)
	}
	episodesListed, err := service.ListEpisodes(firstID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Size != 600 || listed[0].ID != firstID || len(episodesListed) != 2 {
		t.Fatalf("list %+v episodes %+v", listed, episodesListed)
	}
	if listed[0].Name != season {
		t.Fatalf("pack title %q", listed[0].Name)
	}

	if err := db.Exec("UPDATE episodes SET subtitle_index = NULL, subtitle_ext = '' WHERE series_id = ?", series.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := LinkSeriesSubtitles(db, packHash, engine.files); err != nil {
		t.Fatal(err)
	}
	if err := db.Where("series_id = ?", series.ID).Order("file_index").Find(&episodes).Error; err != nil {
		t.Fatal(err)
	}
	if episodes[0].SubtitleIndex == nil || *episodes[0].SubtitleIndex != 2 || episodes[0].SubtitleExt != ".srt" || episodes[1].SubtitleIndex != nil {
		t.Fatalf("relinked %+v", episodes)
	}
	engine.files = nil

	const movieHash = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	legacy := persistence.TorrentModel{Hash: movieHash, Name: "old.mkv", Size: 1}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	legacyMovie := persistence.MovieModel{TorrentID: legacy.ID, FileIndex: 0, Size: 1}
	if err := db.Create(&legacyMovie).Error; err != nil {
		t.Fatal(err)
	}
	engine.videos = []domain.VideoMeta{{
		Index: 4,
		Name:  "bolt_supercao.mkv",
		Path:  "/data/bolt_supercao.mkv",
		Size:  539402598,
	}}
	movieKind, movieID, err := service.AddTorrent(&domain.AddTorrentOptions{
		Magnet: "magnet:?xt=urn:btih:" + movieHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	if movieKind != "movie" || movieID != legacyMovie.ID {
		t.Fatalf("movie %s id %d, want %d", movieKind, movieID, legacyMovie.ID)
	}
	var movie persistence.MovieModel
	if err := db.First(&movie, legacyMovie.ID).Error; err != nil {
		t.Fatal(err)
	}
	var updated persistence.TorrentModel
	if err := db.First(&updated, legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if movie.FileIndex != 4 || movie.Size != 539402598 || updated.Name != "bolt_supercao.mkv" || updated.Size != 539402598 {
		t.Fatalf("updated movie %+v torrent %+v", movie, updated)
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
	files     []domain.VideoMeta
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

func (f *fakeEngine) ListFiles(*domain.MagnetModel) ([]domain.VideoMeta, error) {
	if f.files != nil {
		return f.files, nil
	}
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
	if err := db.AutoMigrate(&persistence.TorrentModel{}, &persistence.MovieModel{}, &persistence.SeriesModel{}, &persistence.EpisodeModel{}); err != nil {
		t.Fatal(err)
	}
	return db
}
