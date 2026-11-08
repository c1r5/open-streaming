package streaming

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	enginedomain "github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestAddTracksReplacesMKVAndWatchReadsIt(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Fatal("ffmpeg is not on PATH")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Fatal("ffprobe is not on PATH")
	}

	dir := t.TempDir()
	moviePath := filepath.Join(dir, "movie.mkv")
	episodePath := filepath.Join(dir, "episode.mkv")
	shortPath := filepath.Join(dir, "short.mkv")
	audioPath := filepath.Join(dir, "voice.m4a")
	writeMKV(t, moviePath)
	copyFile(t, moviePath, episodePath)
	copyFile(t, moviePath, shortPath)
	writeAudio(t, audioPath)
	subtitle := []byte("1\n00:00:00,000 --> 00:00:01,000\nOi\n")
	before, err := os.ReadFile(shortPath)
	if err != nil {
		t.Fatal(err)
	}
	sourceCodec := videoCodec(t, moviePath)

	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}, &persistence.MovieModel{}, &persistence.SeriesModel{}, &persistence.EpisodeModel{}, &persistence.ExtraTrackModel{}); err != nil {
		t.Fatal(err)
	}
	movieTorrent := persistence.TorrentModel{Hash: "movie", Name: "Movie", Size: 1}
	shortTorrent := persistence.TorrentModel{Hash: "short", Name: "Short", Size: 1}
	plainTorrent := persistence.TorrentModel{Hash: "plain", Name: "Plain", Size: 1}
	pack := persistence.TorrentModel{Hash: "pack", Name: "Season", Size: 1}
	for _, row := range []*persistence.TorrentModel{&movieTorrent, &shortTorrent, &plainTorrent, &pack} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	movie := persistence.MovieModel{TorrentID: movieTorrent.ID, FileIndex: 0, Size: 1}
	short := persistence.MovieModel{TorrentID: shortTorrent.ID, FileIndex: 0, Size: 1}
	plain := persistence.MovieModel{TorrentID: plainTorrent.ID, FileIndex: 0, Size: 1}
	for _, row := range []*persistence.MovieModel{&movie, &short, &plain} {
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
		"movie": {path: moviePath, complete: true},
		"short": {path: shortPath, complete: false},
		"plain": {path: filepath.Join(dir, "movie.mp4"), complete: true},
		"pack":  {path: episodePath, complete: true},
	}}
	mux := http.NewServeMux()
	New(mux, engine, db)

	audio, err := os.ReadFile(audioPath)
	if err != nil {
		t.Fatal(err)
	}
	rec := postTracks(t, mux, "/movies/"+id(movie.ID)+"/tracks", audio, "voice.m4a", subtitle, "cap.srt")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	tracks := decodeTracks(t, rec.Body.Bytes())
	if len(tracks) != 2 || tracks[0].Kind != "audio" || tracks[0].Language != "eng" || tracks[0].Title != "Dublagem" || !tracks[0].Default {
		t.Fatalf("tracks %+v", tracks)
	}
	if tracks[1].Kind != "subtitle" || tracks[1].Language != "por" || tracks[1].Title != "Legenda" || !tracks[1].Default {
		t.Fatalf("tracks %+v", tracks)
	}
	if videoCodec(t, moviePath) != sourceCodec {
		t.Fatalf("video codec changed %s", videoCodec(t, moviePath))
	}
	if !hasTrack(t, moviePath, "subtitle", "por", "Legenda", true) || !hasTrack(t, moviePath, "audio", "eng", "Dublagem", true) {
		t.Fatal("muxed streams missing")
	}
	var stored persistence.MovieModel
	if err := db.First(&stored, movie.ID).Error; err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(moviePath)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Size != info.Size() || !stored.LocalPlayback {
		t.Fatalf("movie row size %d playback %v file %d", stored.Size, stored.LocalPlayback, info.Size())
	}
	watch := httptest.NewRecorder()
	mux.ServeHTTP(watch, httptest.NewRequest(http.MethodGet, "/movies/"+id(movie.ID)+"/watch", nil))
	if watch.Code != http.StatusOK || int64(watch.Body.Len()) != info.Size() {
		t.Fatalf("watch %d len %d file %d", watch.Code, watch.Body.Len(), info.Size())
	}

	again := postAudio(t, mux, "/movies/"+id(movie.ID)+"/tracks", audio, "extra.m4a", "spa", "Comentario")
	if again.Code != http.StatusCreated {
		t.Fatalf("second %d %s", again.Code, again.Body.String())
	}
	tracks = decodeTracks(t, again.Body.Bytes())
	if len(tracks) != 3 || tracks[0].Default || !tracks[2].Default || tracks[2].Language != "spa" {
		t.Fatalf("second tracks %+v", tracks)
	}

	shortBody := postAudio(t, mux, "/movies/"+id(short.ID)+"/tracks", audio, "voice.m4a", "eng", "Dublagem")
	if shortBody.Code != http.StatusConflict {
		t.Fatalf("incomplete %d %s", shortBody.Code, shortBody.Body.String())
	}
	got, err := os.ReadFile(shortPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, got) {
		t.Fatal("incomplete file changed")
	}

	plainBody := postAudio(t, mux, "/movies/"+id(plain.ID)+"/tracks", audio, "voice.m4a", "eng", "Dublagem")
	if plainBody.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("not mkv %d %s", plainBody.Code, plainBody.Body.String())
	}

	episodeRec := postTracks(t, mux, "/series/"+id(series.ID)+"/episodes/"+id(episode.ID)+"/tracks", audio, "voice.m4a", subtitle, "cap.srt")
	if episodeRec.Code != http.StatusCreated {
		t.Fatalf("episode %d %s", episodeRec.Code, episodeRec.Body.String())
	}
	episodeInfo, err := os.Stat(episodePath)
	if err != nil {
		t.Fatal(err)
	}
	episodeWatch := httptest.NewRecorder()
	mux.ServeHTTP(episodeWatch, httptest.NewRequest(http.MethodGet, "/series/"+id(series.ID)+"/episodes/"+id(episode.ID)+"/watch", nil))
	if episodeWatch.Code != http.StatusOK || int64(episodeWatch.Body.Len()) != episodeInfo.Size() {
		t.Fatalf("episode watch %d len %d file %d", episodeWatch.Code, episodeWatch.Body.Len(), episodeInfo.Size())
	}
	if !engine.detached["movie"] || !engine.detached["pack"] {
		t.Fatalf("detach %#v", engine.detached)
	}
}

type trackJSON struct {
	ID       uint   `json:"id"`
	Kind     string `json:"kind"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Default  bool   `json:"default"`
}

func decodeTracks(t *testing.T, raw []byte) []trackJSON {
	t.Helper()
	var payload struct {
		Tracks []trackJSON `json:"tracks"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Tracks
}

func postTracks(t *testing.T, handler http.Handler, url string, audio []byte, audioName string, subtitle []byte, subtitleName string) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	writePart(t, form, "audio", audioName, audio)
	if err := form.WriteField("audio_language", "eng"); err != nil {
		t.Fatal(err)
	}
	if err := form.WriteField("audio_title", "Dublagem"); err != nil {
		t.Fatal(err)
	}
	if err := form.WriteField("audio_default", "true"); err != nil {
		t.Fatal(err)
	}
	writePart(t, form, "subtitle", subtitleName, subtitle)
	if err := form.WriteField("subtitle_language", "por"); err != nil {
		t.Fatal(err)
	}
	if err := form.WriteField("subtitle_title", "Legenda"); err != nil {
		t.Fatal(err)
	}
	if err := form.WriteField("subtitle_default", "true"); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func postAudio(t *testing.T, handler http.Handler, url string, audio []byte, name, language, title string) *httptest.ResponseRecorder {
	t.Helper()
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	writePart(t, form, "audio", name, audio)
	if err := form.WriteField("audio_language", language); err != nil {
		t.Fatal(err)
	}
	if err := form.WriteField("audio_title", title); err != nil {
		t.Fatal(err)
	}
	if err := form.WriteField("audio_default", "true"); err != nil {
		t.Fatal(err)
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func writePart(t *testing.T, form *multipart.Writer, field, name string, payload []byte) {
	t.Helper()
	part, err := form.CreateFormFile(field, name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatal(err)
	}
}

func writeMKV(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc=duration=1:size=160x120:rate=10", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", "-shortest", path)
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("mkv: %v %s", err, raw)
	}
}

func writeAudio(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=880:duration=1", "-c:a", "aac", path)
	if raw, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("audio: %v %s", err, raw)
	}
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	raw, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func videoCodec(t *testing.T, path string) string {
	t.Helper()
	for _, stream := range probeStreams(t, path) {
		if stream.CodecType == "video" {
			return stream.CodecName
		}
	}
	t.Fatal("no video")
	return ""
}

func hasTrack(t *testing.T, path, kind, language, title string, def bool) bool {
	t.Helper()
	for _, stream := range probeStreams(t, path) {
		if stream.CodecType != kind {
			continue
		}
		if stream.Tags["language"] == language && stream.Tags["title"] == title && (stream.Disposition["default"] == 1) == def {
			return true
		}
	}
	return false
}

type probedStream struct {
	CodecType   string            `json:"codec_type"`
	CodecName   string            `json:"codec_name"`
	Tags        map[string]string `json:"tags"`
	Disposition map[string]int    `json:"disposition"`
}

func probeStreams(t *testing.T, path string) []probedStream {
	t.Helper()
	cmd := exec.Command("ffprobe", "-v", "quiet", "-print_format", "json", "-show_streams", path)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Streams []probedStream `json:"streams"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Streams
}

type trackFile struct {
	path     string
	complete bool
}

type trackEngine struct {
	files    map[string]trackFile
	detached map[string]bool
}

func (e *trackEngine) LocalFile(hash string, _ int) (string, bool, error) {
	file := e.files[hash]
	return file.path, file.complete, nil
}

func (e *trackEngine) DetachFile(hash string, _ int) error {
	if e.detached == nil {
		e.detached = map[string]bool{}
	}
	e.detached[hash] = true
	return nil
}

func (e *trackEngine) Resolve(string) (*enginedomain.TorrentFile, error) {
	return &enginedomain.TorrentFile{}, nil
}
func (e *trackEngine) ResolveMagnet(*enginedomain.MagnetModel) (*enginedomain.TorrentFile, error) {
	return e.Resolve("")
}
func (e *trackEngine) ResolveFile(string, int) (*enginedomain.TorrentFile, error) {
	return e.Resolve("")
}
func (e *trackEngine) ListVideos(*enginedomain.MagnetModel) ([]enginedomain.VideoMeta, error) {
	return nil, nil
}
func (e *trackEngine) ListFiles(*enginedomain.MagnetModel) ([]enginedomain.VideoMeta, error) {
	return nil, nil
}
func (e *trackEngine) PersistTorrentFile(string) error { return nil }
func (e *trackEngine) Remove(string) error             { return nil }
func (e *trackEngine) RemoveTorrentFile(string) error  { return nil }
func (e *trackEngine) RemoveFile(string, int) error    { return nil }
func (e *trackEngine) IgnoreFiles(string, []int) error { return nil }
func (e *trackEngine) Swarm(string) (enginedomain.SwarmView, bool) {
	return enginedomain.SwarmView{}, false
}
