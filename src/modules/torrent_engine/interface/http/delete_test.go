package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/application"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestDeleteMovieAndSeriesRemovesListRowAndTorrent(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}, &persistence.MovieModel{}, &persistence.SeriesModel{}, &persistence.EpisodeModel{}); err != nil {
		t.Fatal(err)
	}

	keepTorrent := persistence.TorrentModel{Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Name: "keep.mkv", Size: 10}
	dropTorrent := persistence.TorrentModel{Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", Name: "drop.mkv", Size: 20}
	seriesTorrent := persistence.TorrentModel{Hash: "cccccccccccccccccccccccccccccccccccccccc", Name: "Show", Size: 30}
	for _, row := range []*persistence.TorrentModel{&keepTorrent, &dropTorrent, &seriesTorrent} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	keep := persistence.MovieModel{TorrentID: keepTorrent.ID, FileIndex: 0, Size: 10}
	drop := persistence.MovieModel{TorrentID: dropTorrent.ID, FileIndex: 0, Size: 20}
	for _, row := range []*persistence.MovieModel{&keep, &drop} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	series := persistence.SeriesModel{TorrentID: seriesTorrent.ID}
	if err := db.Create(&series).Error; err != nil {
		t.Fatal(err)
	}
	episode := persistence.EpisodeModel{SeriesID: series.ID, FileIndex: 1, Name: "e01.mkv", Size: 30}
	if err := db.Create(&episode).Error; err != nil {
		t.Fatal(err)
	}

	engine := &deleteEngine{}
	controller := NewController(application.NewService(&application.ServiceOptions{DB: db, TorrentEngine: engine}))

	status, body := deleteID(t, controller.DeleteMovieHandler(), drop.ID)
	if status != http.StatusNoContent || body != "" {
		t.Fatalf("delete movie status %d body %q", status, body)
	}
	if len(engine.removed) != 1 || engine.removed[0] != dropTorrent.Hash || len(engine.sidecars) != 1 || engine.sidecars[0] != dropTorrent.Hash {
		t.Fatalf("engine removed %#v sidecars %#v", engine.removed, engine.sidecars)
	}
	movies := listMovies(t, controller)
	if len(movies) != 1 || movies[0].ID != keep.ID || movies[0].Name != "keep.mkv" {
		t.Fatalf("movies after delete %+v", movies)
	}

	engine.fail = errors.New("disk")
	status, body = deleteID(t, controller.DeleteMovieHandler(), keep.ID)
	if status != http.StatusInternalServerError || body != `{"error":"disk"}`+"\n" {
		t.Fatalf("failed delete status %d body %q", status, body)
	}
	movies = listMovies(t, controller)
	if len(movies) != 1 || movies[0].ID != keep.ID {
		t.Fatalf("movie removed after disk error %+v", movies)
	}
	if len(engine.removed) != 1 || len(engine.sidecars) != 1 {
		t.Fatalf("engine changed on failure %#v %#v", engine.removed, engine.sidecars)
	}

	status, body = deleteID(t, controller.DeleteMovieHandler(), 999)
	if status != http.StatusNotFound || body != `{"error":"movie not found"}`+"\n" {
		t.Fatalf("missing movie status %d body %q", status, body)
	}

	engine.fail = nil
	status, body = deleteID(t, controller.DeleteSeriesHandler(), series.ID)
	if status != http.StatusNoContent || body != "" {
		t.Fatalf("delete series status %d body %q", status, body)
	}
	if engine.removed[len(engine.removed)-1] != seriesTorrent.Hash || engine.sidecars[len(engine.sidecars)-1] != seriesTorrent.Hash {
		t.Fatalf("series engine %#v %#v", engine.removed, engine.sidecars)
	}
	seriesRows := listSeries(t, controller)
	if len(seriesRows) != 0 {
		t.Fatalf("series left %+v", seriesRows)
	}
	status, body = episodes(t, controller, series.ID)
	if status != http.StatusNotFound || body != `{"error":"series not found"}`+"\n" {
		t.Fatalf("episodes after series delete status %d body %q", status, body)
	}

	status, body = deleteID(t, controller.DeleteSeriesHandler(), series.ID)
	if status != http.StatusNotFound || body != `{"error":"series not found"}`+"\n" {
		t.Fatalf("second series delete status %d body %q", status, body)
	}
}

func TestDeleteEpisodeRemovesFileUntilTheLastOneRemovesTheSeries(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&persistence.TorrentModel{}, &persistence.MovieModel{}, &persistence.SeriesModel{}, &persistence.EpisodeModel{}); err != nil {
		t.Fatal(err)
	}
	torrent := persistence.TorrentModel{Hash: "dddddddddddddddddddddddddddddddddddddddd", Name: "Show", Size: 30}
	if err := db.Create(&torrent).Error; err != nil {
		t.Fatal(err)
	}
	series := persistence.SeriesModel{TorrentID: torrent.ID}
	if err := db.Create(&series).Error; err != nil {
		t.Fatal(err)
	}
	subtitle := 2
	first := persistence.EpisodeModel{SeriesID: series.ID, FileIndex: 1, Name: "e01.mkv", Size: 10, SubtitleIndex: &subtitle, SubtitleExt: ".srt"}
	second := persistence.EpisodeModel{SeriesID: series.ID, FileIndex: 3, Name: "e02.mkv", Size: 20}
	for _, row := range []*persistence.EpisodeModel{&first, &second} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	engine := &deleteEngine{}
	controller := NewController(application.NewService(&application.ServiceOptions{DB: db, TorrentEngine: engine}))

	engine.fail = errors.New("disk")
	status, body := deleteEpisode(t, controller, series.ID, first.ID)
	if status != http.StatusInternalServerError || body != `{"error":"disk"}`+"\n" {
		t.Fatalf("failed episode delete status %d body %q", status, body)
	}
	left := listedEpisodes(t, controller, series.ID)
	if len(left) != 2 {
		t.Fatalf("episodes after disk error %+v", left)
	}
	if len(engine.files) != 0 || len(engine.removed) != 0 {
		t.Fatalf("engine changed on failure %#v %#v", engine.files, engine.removed)
	}

	engine.fail = nil
	status, body = deleteEpisode(t, controller, series.ID, first.ID)
	if status != http.StatusNoContent || body != "" {
		t.Fatalf("delete episode status %d body %q", status, body)
	}
	if len(engine.removed) != 0 || len(engine.sidecars) != 0 || len(engine.files) != 2 || engine.files[0] != (removedFile{torrent.Hash, 1}) || engine.files[1] != (removedFile{torrent.Hash, 2}) {
		t.Fatalf("partial engine removed %#v sidecars %#v files %#v", engine.removed, engine.sidecars, engine.files)
	}
	left = listedEpisodes(t, controller, series.ID)
	if len(left) != 1 || left[0].ID != second.ID || left[0].Name != "e02.mkv" {
		t.Fatalf("episodes left %+v", left)
	}
	seriesRows := listSeries(t, controller)
	if len(seriesRows) != 1 || seriesRows[0].ID != series.ID {
		t.Fatalf("series after one episode %+v", seriesRows)
	}

	status, body = deleteEpisode(t, controller, series.ID, 999)
	if status != http.StatusNotFound || body != `{"error":"episode not found"}`+"\n" {
		t.Fatalf("missing episode status %d body %q", status, body)
	}

	status, body = deleteEpisode(t, controller, series.ID, second.ID)
	if status != http.StatusNoContent || body != "" {
		t.Fatalf("delete last episode status %d body %q", status, body)
	}
	if len(engine.removed) != 1 || engine.removed[0] != torrent.Hash || len(engine.sidecars) != 1 || engine.sidecars[0] != torrent.Hash {
		t.Fatalf("last episode engine %#v %#v", engine.removed, engine.sidecars)
	}
	if rows := listSeries(t, controller); len(rows) != 0 {
		t.Fatalf("series left %+v", rows)
	}
	status, body = episodes(t, controller, series.ID)
	if status != http.StatusNotFound || body != `{"error":"series not found"}`+"\n" {
		t.Fatalf("episodes after last delete status %d body %q", status, body)
	}
}

func deleteEpisode(t *testing.T, controller *Controller, seriesID, episodeID uint) (int, string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodDelete, "/", nil)
	request.SetPathValue("id", strconv.FormatUint(uint64(seriesID), 10))
	request.SetPathValue("episodeId", strconv.FormatUint(uint64(episodeID), 10))
	recorder := httptest.NewRecorder()
	controller.DeleteEpisodeHandler()(recorder, request)
	return recorder.Code, recorder.Body.String()
}

func listedEpisodes(t *testing.T, controller *Controller, seriesID uint) []application.ListedEpisode {
	t.Helper()
	status, body := episodes(t, controller, seriesID)
	if status != http.StatusOK {
		t.Fatalf("list episodes %d %s", status, body)
	}
	var items []application.ListedEpisode
	if err := json.Unmarshal([]byte(body), &items); err != nil {
		t.Fatal(err)
	}
	return items
}

func deleteID(t *testing.T, handler func(http.ResponseWriter, *http.Request), id uint) (int, string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodDelete, "/", nil)
	request.SetPathValue("id", strconv.FormatUint(uint64(id), 10))
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder.Code, recorder.Body.String()
}

func listMovies(t *testing.T, controller *Controller) []application.ListedMovie {
	t.Helper()
	recorder := httptest.NewRecorder()
	controller.MoviesHandler()(recorder, httptest.NewRequest(http.MethodGet, "/movies", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("list movies %d %s", recorder.Code, recorder.Body.String())
	}
	var items []application.ListedMovie
	if err := json.Unmarshal(recorder.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	return items
}

func listSeries(t *testing.T, controller *Controller) []application.ListedSeries {
	t.Helper()
	recorder := httptest.NewRecorder()
	controller.SeriesHandler()(recorder, httptest.NewRequest(http.MethodGet, "/series", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("list series %d %s", recorder.Code, recorder.Body.String())
	}
	var items []application.ListedSeries
	if err := json.Unmarshal(recorder.Body.Bytes(), &items); err != nil {
		t.Fatal(err)
	}
	return items
}

func episodes(t *testing.T, controller *Controller, id uint) (int, string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.SetPathValue("id", strconv.FormatUint(uint64(id), 10))
	recorder := httptest.NewRecorder()
	controller.EpisodesHandler()(recorder, request)
	return recorder.Code, recorder.Body.String()
}

type removedFile struct {
	hash  string
	index int
}

type deleteEngine struct {
	removed  []string
	sidecars []string
	files    []removedFile
	fail     error
}

func (e *deleteEngine) Resolve(string) (*domain.TorrentFile, error) {
	return &domain.TorrentFile{}, nil
}
func (e *deleteEngine) ResolveMagnet(*domain.MagnetModel) (*domain.TorrentFile, error) {
	return &domain.TorrentFile{}, nil
}
func (e *deleteEngine) ResolveFile(string, int) (*domain.TorrentFile, error) {
	return &domain.TorrentFile{}, nil
}
func (e *deleteEngine) ListVideos(*domain.MagnetModel) ([]domain.VideoMeta, error) {
	return nil, nil
}
func (e *deleteEngine) ListFiles(*domain.MagnetModel) ([]domain.VideoMeta, error) {
	return nil, nil
}
func (e *deleteEngine) PersistTorrentFile(string) error { return nil }
func (e *deleteEngine) RemoveFile(hash string, index int) error {
	if e.fail != nil {
		return e.fail
	}
	e.files = append(e.files, removedFile{hash, index})
	return nil
}

func (e *deleteEngine) IgnoreFiles(string, []int) error { return nil }

func (e *deleteEngine) Swarm(string) (domain.SwarmView, bool) { return domain.SwarmView{}, false }

func (e *deleteEngine) Remove(hash string) error {
	if e.fail != nil {
		return e.fail
	}
	e.removed = append(e.removed, hash)
	return nil
}
func (e *deleteEngine) RemoveTorrentFile(hash string) error {
	e.sidecars = append(e.sidecars, hash)
	return nil
}
