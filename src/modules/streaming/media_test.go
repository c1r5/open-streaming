package streaming

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestMediaListsEmbeddedTracks(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Fatal("ffmpeg is not on PATH")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Fatal("ffprobe is not on PATH")
	}

	dir := t.TempDir()
	moviePath := filepath.Join(dir, "movie.mkv")
	episodePath := filepath.Join(dir, "episode.mkv")
	writeTaggedMKV(t, moviePath)
	copyFile(t, moviePath, episodePath)
	brokenPath := filepath.Join(dir, "broken.mkv")
	if err := os.WriteFile(brokenPath, []byte("not a matroska file"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}, &persistence.MovieModel{}, &persistence.SeriesModel{}, &persistence.EpisodeModel{}); err != nil {
		t.Fatal(err)
	}
	movieTorrent := persistence.TorrentModel{Hash: "movie", Name: "Movie.mkv", Size: 1}
	plainTorrent := persistence.TorrentModel{Hash: "plain", Name: "Plain.mp4", Size: 1}
	shortTorrent := persistence.TorrentModel{Hash: "short", Name: "Short.mkv", Size: 1}
	brokenTorrent := persistence.TorrentModel{Hash: "broken", Name: "Broken.mkv", Size: 1}
	pack := persistence.TorrentModel{Hash: "pack", Name: "Season", Size: 1}
	for _, row := range []*persistence.TorrentModel{&movieTorrent, &plainTorrent, &shortTorrent, &brokenTorrent, &pack} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	movie := persistence.MovieModel{TorrentID: movieTorrent.ID, FileIndex: 0, Size: 1}
	plain := persistence.MovieModel{TorrentID: plainTorrent.ID, FileIndex: 0, Size: 1}
	short := persistence.MovieModel{TorrentID: shortTorrent.ID, FileIndex: 0, Size: 1}
	broken := persistence.MovieModel{TorrentID: brokenTorrent.ID, FileIndex: 0, Size: 1}
	for _, row := range []*persistence.MovieModel{&movie, &plain, &short, &broken} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	series := persistence.SeriesModel{TorrentID: pack.ID}
	if err := db.Create(&series).Error; err != nil {
		t.Fatal(err)
	}
	episode := persistence.EpisodeModel{SeriesID: series.ID, FileIndex: 3, Name: "S01E01.mkv", Size: 1}
	if err := db.Create(&episode).Error; err != nil {
		t.Fatal(err)
	}

	engine := &trackEngine{files: map[string]trackFile{
		"movie":  {path: moviePath, complete: true},
		"plain":  {path: filepath.Join(dir, "movie.mp4"), complete: true},
		"short":  {path: filepath.Join(dir, "short.mkv"), complete: false},
		"broken": {path: brokenPath, complete: true},
		"pack":   {path: episodePath, complete: true},
	}}
	mux := http.NewServeMux()
	New(mux, engine, db)

	movieRec := getMedia(t, mux, "/movies/"+id(movie.ID)+"/media")
	if movieRec.Code != http.StatusOK {
		t.Fatalf("movie %d %s", movieRec.Code, movieRec.Body.String())
	}
	movieMedia := decodeMedia(t, movieRec.Body.Bytes())
	assertTaggedMedia(t, movieMedia)

	episodeRec := getMedia(t, mux, "/series/"+id(series.ID)+"/episodes/"+id(episode.ID)+"/media")
	if episodeRec.Code != http.StatusOK {
		t.Fatalf("episode %d %s", episodeRec.Code, episodeRec.Body.String())
	}
	assertTaggedMedia(t, decodeMedia(t, episodeRec.Body.Bytes()))

	plainRec := getMedia(t, mux, "/movies/"+id(plain.ID)+"/media")
	if plainRec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("plain %d %s", plainRec.Code, plainRec.Body.String())
	}
	shortRec := getMedia(t, mux, "/movies/"+id(short.ID)+"/media")
	if shortRec.Code != http.StatusConflict {
		t.Fatalf("short %d %s", shortRec.Code, shortRec.Body.String())
	}
	missingRec := getMedia(t, mux, "/movies/999999/media")
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("missing %d %s", missingRec.Code, missingRec.Body.String())
	}
	brokenRec := getMedia(t, mux, "/movies/"+id(broken.ID)+"/media")
	if brokenRec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("broken %d %s", brokenRec.Code, brokenRec.Body.String())
	}
}

func writeTaggedMKV(t *testing.T, path string) {
	t.Helper()
	subtitle := filepath.Join(t.TempDir(), "cap.srt")
	if err := os.WriteFile(subtitle, []byte("1\n00:00:00,000 --> 00:00:01,000\nOi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=160x120:rate=10",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-i", subtitle,
		"-map", "0:v", "-map", "1:a", "-map", "2:s",
		"-c:v", "libx264", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-ac", "2",
		"-c:s", "srt",
		"-metadata:s:a:0", "language=por",
		"-metadata:s:a:0", "title=Dublagem",
		"-disposition:a:0", "default",
		"-metadata:s:s:0", "language=eng",
		"-metadata:s:s:0", "title=English",
		"-disposition:s:0", "0",
		"-shortest", path,
	)
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tagged mkv: %v %s", err, raw)
	}
}

func getMedia(t *testing.T, handler http.Handler, url string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

type mediaBody struct {
	DurationSeconds float64 `json:"duration_seconds"`
	Video           *struct {
		Codec  string `json:"codec"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"video"`
	Audios []struct {
		Index    int    `json:"index"`
		Codec    string `json:"codec"`
		Language string `json:"language"`
		Title    string `json:"title"`
		Channels int    `json:"channels"`
		Default  bool   `json:"default"`
	} `json:"audios"`
	Subtitles []struct {
		Index    int    `json:"index"`
		Codec    string `json:"codec"`
		Language string `json:"language"`
		Title    string `json:"title"`
		Default  bool   `json:"default"`
	} `json:"subtitles"`
}

func decodeMedia(t *testing.T, raw []byte) mediaBody {
	t.Helper()
	var body mediaBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func assertTaggedMedia(t *testing.T, body mediaBody) {
	t.Helper()
	if body.DurationSeconds <= 0 {
		t.Fatalf("duration %v", body.DurationSeconds)
	}
	if body.Video == nil || body.Video.Codec != "h264" || body.Video.Width != 160 || body.Video.Height != 120 {
		t.Fatalf("video %+v", body.Video)
	}
	audio := body.Audios
	if len(audio) != 1 || audio[0].Index < 1 || audio[0].Codec != "aac" || audio[0].Language != "por" || audio[0].Title != "Dublagem" || audio[0].Channels != 2 || !audio[0].Default {
		t.Fatalf("audio %+v", audio)
	}
	if len(body.Subtitles) != 1 || body.Subtitles[0].Codec != "subrip" || body.Subtitles[0].Language != "eng" || body.Subtitles[0].Title != "English" || body.Subtitles[0].Default {
		t.Fatalf("subtitle %+v", body.Subtitles)
	}
}
