package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/application"
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
	if err := db.AutoMigrate(&persistence.TorrentModel{}); err != nil {
		t.Fatal(err)
	}

	older := persistence.TorrentModel{Hash: "aaa", TorrentName: "older.mkv", Size: 10}
	newer := persistence.TorrentModel{Hash: "bbb", TorrentName: "newer.mkv", Size: 20}
	removed := persistence.TorrentModel{Hash: "ccc", TorrentName: "gone.mkv", Size: 30}
	imdb := "tt1"
	newer.IMDB = &imdb
	for _, row := range []*persistence.TorrentModel{&older, &newer, &removed} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	db.Model(&older).Update("created_at", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
	db.Model(&newer).Update("created_at", time.Date(2024, 3, 4, 0, 0, 0, 0, time.UTC))
	if err := db.Delete(&removed).Error; err != nil {
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

func TestListHandler_empty(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}); err != nil {
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
