package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	catalogdomain "github.com/c1r5/open-streaming/src/modules/catalog/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/application"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/cache"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestAddStatus_metadataThenReadyOnce(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}, &persistence.MovieModel{}, &persistence.SeriesModel{}, &persistence.EpisodeModel{}); err != nil {
		t.Fatal(err)
	}

	const hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	engine := &statusEngine{
		started: make(chan struct{}),
		release: make(chan struct{}),
		videos:  []domain.VideoMeta{{Index: 0, Name: "filme.mkv", Path: "/data/filme.mkv", Size: 42}},
	}
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(engine.release) }) }
	t.Cleanup(release)

	service := application.NewService(&application.ServiceOptions{
		SearchCache:   cache.NewCache[*catalogdomain.TorrentSearchResult](time.Minute),
		TorrentEngine: engine,
		DB:            db,
	})
	controller := NewController(service)

	status, body := callAdd(t, controller, `{}`)
	if status != http.StatusBadRequest || body != `{"error":"hash or magnet required"}`+"\n" {
		t.Fatalf("empty body %d %q", status, body)
	}
	status, body = callAdd(t, controller, `{"magnet":"nope"}`)
	if status != http.StatusBadRequest || !strings.Contains(body, `"error":"magnet:`) {
		t.Fatalf("bad magnet %d %q", status, body)
	}
	status, body = callAdd(t, controller, `{"hash":"abc"}`)
	if status != http.StatusBadRequest || body != `{"error":"invalid hash"}`+"\n" {
		t.Fatalf("short hash %d %q", status, body)
	}
	missing := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	status, body = callAdd(t, controller, `{"hash":"`+missing+`"}`)
	if status != http.StatusBadRequest || body != `{"error":"failed to get torrent from cache: cache: key not found"}`+"\n" {
		t.Fatalf("cache miss %d %q", status, body)
	}

	raw := callStatusRaw(t, controller, "xyz")
	if raw.code != http.StatusBadRequest || raw.body != `{"error":"invalid hash"}`+"\n" {
		t.Fatalf("bad status hash %d %q", raw.code, raw.body)
	}
	raw = callStatusRaw(t, controller, "dddddddddddddddddddddddddddddddddddddddd")
	if raw.code != http.StatusNotFound || raw.body != `{"error":"torrent not found"}`+"\n" {
		t.Fatalf("unknown hash %d %q", raw.code, raw.body)
	}

	engine.setSwarm(2, "")
	status, got := callAddStatus(t, controller, `{"magnet":"magnet:?xt=urn:btih:`+hash+`"}`)
	if status != http.StatusAccepted {
		t.Fatalf("first add %d %+v", status, got)
	}
	if got.Hash != hash || got.Phase != "metadata" || got.Peers != 2 || got.Name != "" || got.Kind != "" || got.ID != 0 || got.Error != "" {
		t.Fatalf("first status %+v", got)
	}
	select {
	case <-engine.started:
	case <-time.After(2 * time.Second):
		t.Fatal("metadata did not start")
	}
	if engine.listCalls() != 1 {
		t.Fatalf("list calls %d", engine.listCalls())
	}
	var rows int64
	if err := db.Model(&persistence.TorrentModel{}).Where("lower(hash) = ?", hash).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("rows before metadata %d", rows)
	}

	status, got = callAddStatus(t, controller, `{"magnet":"magnet:?xt=urn:btih:`+hash+`"}`)
	if status != http.StatusOK || got.Phase != "metadata" || got.Hash != hash || engine.listCalls() != 1 {
		t.Fatalf("repeat add %d %+v calls %d", status, got, engine.listCalls())
	}

	engine.setSwarm(5, "Ao vivo")
	status, got = callStatus(t, controller, hash)
	if status != http.StatusOK || got.Phase != "metadata" || got.Peers != 5 || got.Name != "Ao vivo" {
		t.Fatalf("poll %+v status %d", got, status)
	}

	engine.setSwarm(5, "")
	release()
	deadline := time.Now().Add(2 * time.Second)
	for {
		status, got = callStatus(t, controller, hash)
		if status == http.StatusOK && got.Phase == "ready" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ready status %d %+v", status, got)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got.Kind != "movie" || got.ID == 0 || got.Name != "filme.mkv" || got.Error != "" || got.Peers != 5 {
		t.Fatalf("ready %+v", got)
	}
	if err := db.Model(&persistence.TorrentModel{}).Where("lower(hash) = ?", hash).Count(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("rows %d", rows)
	}
	var movie persistence.MovieModel
	if err := db.First(&movie, got.ID).Error; err != nil {
		t.Fatal(err)
	}
	if movie.Size != 42 {
		t.Fatalf("movie %+v", movie)
	}

	status, again := callAddStatus(t, controller, `{"magnet":"magnet:?xt=urn:btih:`+hash+`"}`)
	if status != http.StatusOK || again.Phase != "ready" || again.Kind != "movie" || again.ID != got.ID || engine.listCalls() != 1 {
		t.Fatalf("saved add %d %+v calls %d", status, again, engine.listCalls())
	}
}

type recorded struct {
	code int
	body string
}

func callAdd(t *testing.T, controller *Controller, payload string) (int, string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/torrent/add", bytes.NewBufferString(payload))
	recorder := httptest.NewRecorder()
	controller.AddHandler()(recorder, request)
	return recorder.Code, recorder.Body.String()
}

func callAddStatus(t *testing.T, controller *Controller, payload string) (int, application.FetchStatus) {
	t.Helper()
	code, body := callAdd(t, controller, payload)
	var status application.FetchStatus
	if err := json.Unmarshal([]byte(body), &status); err != nil {
		t.Fatalf("add body %q: %v", body, err)
	}
	return code, status
}

func callStatus(t *testing.T, controller *Controller, hash string) (int, application.FetchStatus) {
	t.Helper()
	raw := callStatusRaw(t, controller, hash)
	var status application.FetchStatus
	if err := json.Unmarshal([]byte(raw.body), &status); err != nil {
		t.Fatalf("status body %q: %v", raw.body, err)
	}
	return raw.code, status
}

func callStatusRaw(t *testing.T, controller *Controller, hash string) recorded {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/torrent/status/"+hash, nil)
	request.SetPathValue("hash", hash)
	recorder := httptest.NewRecorder()
	controller.StatusHandler()(recorder, request)
	return recorded{code: recorder.Code, body: recorder.Body.String()}
}

type statusEngine struct {
	videos  []domain.VideoMeta
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
	mu      sync.Mutex
	peers   int
	name    string
}

func (e *statusEngine) setSwarm(peers int, name string) {
	e.mu.Lock()
	e.peers = peers
	e.name = name
	e.mu.Unlock()
}

func (e *statusEngine) listCalls() int { return int(e.calls.Load()) }

func (e *statusEngine) Resolve(string) (*domain.TorrentFile, error) {
	return &domain.TorrentFile{}, nil
}
func (e *statusEngine) ResolveMagnet(*domain.MagnetModel) (*domain.TorrentFile, error) {
	return &domain.TorrentFile{}, nil
}
func (e *statusEngine) ResolveFile(string, int) (*domain.TorrentFile, error) {
	return &domain.TorrentFile{}, nil
}
func (e *statusEngine) ListVideos(*domain.MagnetModel) ([]domain.VideoMeta, error) {
	if e.calls.Add(1) == 1 {
		close(e.started)
	}
	<-e.release
	return e.videos, nil
}
func (e *statusEngine) ListFiles(*domain.MagnetModel) ([]domain.VideoMeta, error) {
	return e.videos, nil
}
func (e *statusEngine) PersistTorrentFile(string) error { return nil }
func (e *statusEngine) Remove(string) error             { return nil }
func (e *statusEngine) RemoveTorrentFile(string) error  { return nil }
func (e *statusEngine) RemoveFile(string, int) error    { return nil }
func (e *statusEngine) IgnoreFiles(string, []int) error { return nil }
func (e *statusEngine) Swarm(string) (domain.SwarmView, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return domain.SwarmView{Peers: e.peers, Name: e.name}, true
}
func (e *statusEngine) LocalFile(string, int) (string, bool, error) { return "", false, nil }
func (e *statusEngine) DetachFile(string, int) error                 { return nil }
