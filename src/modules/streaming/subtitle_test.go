package streaming

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	enginedomain "github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestEpisodeSubtitle_servesRangeOr404(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}, &persistence.MovieModel{}, &persistence.SeriesModel{}, &persistence.EpisodeModel{}); err != nil {
		t.Fatal(err)
	}
	pack := persistence.TorrentModel{Hash: "pack", Name: "Season", Size: 1}
	if err := db.Create(&pack).Error; err != nil {
		t.Fatal(err)
	}
	series := persistence.SeriesModel{TorrentID: pack.ID}
	if err := db.Create(&series).Error; err != nil {
		t.Fatal(err)
	}
	subtitleIndex := 4
	withSub := persistence.EpisodeModel{SeriesID: series.ID, FileIndex: 3, Name: "S01E01.mkv", Size: 10, SubtitleIndex: &subtitleIndex, SubtitleExt: ".srt"}
	without := persistence.EpisodeModel{SeriesID: series.ID, FileIndex: 7, Name: "S01E02.mkv", Size: 10}
	if err := db.Create(&withSub).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&without).Error; err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	New(mux, &subtitleEngine{}, db)

	body := "1\n00:00:01,000 --> 00:00:02,000\nOi\n"
	okReq := httptest.NewRequest(http.MethodGet, "/series/"+id(series.ID)+"/episodes/"+id(withSub.ID)+"/subtitle", nil)
	okReq.Header.Set("Range", "bytes=0-0")
	okRec := httptest.NewRecorder()
	mux.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusPartialContent {
		t.Fatalf("status %d body %s", okRec.Code, okRec.Body.String())
	}
	if got := okRec.Header().Get("Content-Type"); got != "text/srt; charset=utf-8" {
		t.Fatalf("content type %q", got)
	}
	got, err := io.ReadAll(okRec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body[:1] {
		t.Fatalf("range body %q", got)
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/series/"+id(series.ID)+"/episodes/"+id(without.ID)+"/subtitle", nil)
	missingRec := httptest.NewRecorder()
	mux.ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("missing status %d", missingRec.Code)
	}
}

func id(value uint) string { return strconv.FormatUint(uint64(value), 10) }

type subtitleEngine struct{}

func (e *subtitleEngine) Resolve(string) (*enginedomain.TorrentFile, error) {
	return &enginedomain.TorrentFile{}, nil
}

func (e *subtitleEngine) ResolveMagnet(*enginedomain.MagnetModel) (*enginedomain.TorrentFile, error) {
	return e.Resolve("")
}

func (e *subtitleEngine) ResolveFile(string, int) (*enginedomain.TorrentFile, error) {
	body := "1\n00:00:01,000 --> 00:00:02,000\nOi\n"
	return &enginedomain.TorrentFile{Path: "S01E01.srt", Name: "S01E01.srt", Reader: strings.NewReader(body)}, nil
}

func (e *subtitleEngine) ListVideos(*enginedomain.MagnetModel) ([]enginedomain.VideoMeta, error) {
	return nil, nil
}

func (e *subtitleEngine) ListFiles(*enginedomain.MagnetModel) ([]enginedomain.VideoMeta, error) {
	return nil, nil
}

func (e *subtitleEngine) PersistTorrentFile(string) error { return nil }
