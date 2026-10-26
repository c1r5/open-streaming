package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/application"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestListHandler_newestFirstAndSkipsRemoved(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}, &persistence.MovieModel{}, &persistence.SeriesModel{}, &persistence.EpisodeModel{}); err != nil {
		t.Fatal(err)
	}

	older := persistence.TorrentModel{Hash: "aaa", Name: "older.mkv", Size: 10}
	newer := persistence.TorrentModel{Hash: "bbb", Name: "newer.mkv", Size: 20}
	removed := persistence.TorrentModel{Hash: "ccc", Name: "gone.mkv", Size: 30}
	for _, row := range []*persistence.TorrentModel{&older, &newer, &removed} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	imdb := "tt1"
	olderMovie := persistence.MovieModel{TorrentID: older.ID, Size: 10}
	newerMovie := persistence.MovieModel{TorrentID: newer.ID, Size: 20, IMDB: &imdb}
	removedMovie := persistence.MovieModel{TorrentID: removed.ID, Size: 30}
	for _, row := range []*persistence.MovieModel{&olderMovie, &newerMovie, &removedMovie} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	db.Model(&older).Update("created_at", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	db.Model(&newer).Update("created_at", time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC))
	if err := db.Delete(&removedMovie).Error; err != nil {
		t.Fatal(err)
	}

	service := application.NewService(&application.ServiceOptions{DB: db})
	controller := NewController(service)
	request := httptest.NewRequest(http.MethodGet, "/torrent", nil)
	recorder := httptest.NewRecorder()
	controller.ListHandler()(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d body %s", recorder.Code, recorder.Body.String())
	}

	var items []application.ListedTorrent
	if err := json.Unmarshal(recorder.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len %d", len(items))
	}
	if items[0].Name != "newer.mkv" || items[0].Size != 20 || items[0].IMDB != "tt1" || items[0].ID == 0 {
		t.Fatalf("first item %+v", items[0])
	}
	if items[1].Name != "older.mkv" || items[1].IMDB != "" {
		t.Fatalf("second item %+v", items[1])
	}
	if !items[0].CreatedAt.After(items[1].CreatedAt) {
		t.Fatalf("order %+v then %+v", items[0].CreatedAt, items[1].CreatedAt)
	}
}

func TestListHandler_groupsSeasonPack(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}, &persistence.MovieModel{}, &persistence.SeriesModel{}, &persistence.EpisodeModel{}); err != nil {
		t.Fatal(err)
	}

	const season = "Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV"
	pack := persistence.TorrentModel{Hash: "pack", Name: season, Size: 600}
	bolt := persistence.TorrentModel{Hash: "bolt", Name: "bolt_supercao.mkv", Size: 10}
	for _, row := range []*persistence.TorrentModel{&pack, &bolt} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	series := persistence.SeriesModel{TorrentID: pack.ID}
	if err := db.Create(&series).Error; err != nil {
		t.Fatal(err)
	}
	episodeTwo := persistence.EpisodeModel{
		SeriesID:  series.ID,
		FileIndex: 7,
		Name:      "The.Big.Bang.Theory.S01E02.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.mkv",
		Size:      500,
	}
	episodeOne := persistence.EpisodeModel{
		SeriesID:  series.ID,
		FileIndex: 3,
		Name:      "The.Big.Bang.Theory.S01E01.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.mkv",
		Size:      100,
	}
	movie := persistence.MovieModel{TorrentID: bolt.ID, Size: 10}
	for _, row := range []any{&episodeTwo, &episodeOne, &movie} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	service := application.NewService(&application.ServiceOptions{DB: db})
	controller := NewController(service)
	recorder := httptest.NewRecorder()
	controller.ListHandler()(recorder, httptest.NewRequest(http.MethodGet, "/torrent", nil))

	var items []application.ListedTorrent
	if err := json.Unmarshal(recorder.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len %d body %s", len(items), recorder.Body.String())
	}
	var packItem, movieItem application.ListedTorrent
	for _, item := range items {
		if item.Name == "bolt_supercao.mkv" {
			movieItem = item
			continue
		}
		packItem = item
	}
	if packItem.Name != season || packItem.Size != 600 || packItem.ID != domain.EpisodeWatchID(episodeOne.ID) {
		t.Fatalf("pack %+v", packItem)
	}
	if len(packItem.Episodes) != 2 || packItem.Episodes[0].ID != domain.EpisodeWatchID(episodeOne.ID) || packItem.Episodes[1].ID != domain.EpisodeWatchID(episodeTwo.ID) {
		t.Fatalf("episodes %+v", packItem.Episodes)
	}
	if packItem.Episodes[0].Name != "The.Big.Bang.Theory.S01E01.720p.BluRay.x264.DUAL-WWW.BLUDV.TV.mkv" {
		t.Fatalf("episode name %q", packItem.Episodes[0].Name)
	}
	if movieItem.Name != "bolt_supercao.mkv" || movieItem.Episodes != nil {
		t.Fatalf("movie %+v", movieItem)
	}
}

func TestListHandler_empty(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}, &persistence.MovieModel{}, &persistence.SeriesModel{}, &persistence.EpisodeModel{}); err != nil {
		t.Fatal(err)
	}

	service := application.NewService(&application.ServiceOptions{DB: db})
	controller := NewController(service)
	recorder := httptest.NewRecorder()
	controller.ListHandler()(recorder, httptest.NewRequest(http.MethodGet, "/torrent", nil))

	if recorder.Body.String() != "[]\n" {
		t.Fatalf("body %q", recorder.Body.String())
	}
}
