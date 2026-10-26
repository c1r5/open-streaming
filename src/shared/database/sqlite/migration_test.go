package sqlite

import (
	"testing"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	"github.com/c1r5/open-streaming/src/shared/database"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
)

func TestMigrationReplacesHashUnique(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE torrents (
			id integer PRIMARY KEY AUTOINCREMENT,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime,
			file_index integer,
			hash text,
			torrent_name text,
			imdb text,
			output text,
			size integer
		)`,
		`CREATE UNIQUE INDEX idx_torrents_hash ON torrents(hash)`,
		`INSERT INTO torrents (hash, file_index, torrent_name, size) VALUES ('abc', 0, 'a.mkv', 1)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := database.ApplyOne(db, 2); err != nil {
		t.Fatal(err)
	}

	if err := db.Exec(`INSERT INTO torrents (hash, file_index, torrent_name, size) VALUES ('abc', 3, 'b.mkv', 2)`).Error; err != nil {
		t.Fatalf("second file of the same torrent: %v", err)
	}
	if err := db.Exec(`INSERT INTO torrents (hash, file_index, torrent_name, size) VALUES ('abc', 3, 'again.mkv', 2)`).Error; err == nil {
		t.Fatal("duplicate hash and file index was stored")
	}
}

func TestMigrationSplitsMoviesAndSeries(t *testing.T) {
	db, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE torrents (
		id integer PRIMARY KEY AUTOINCREMENT,
		created_at datetime,
		updated_at datetime,
		deleted_at datetime,
		file_index integer,
		hash text,
		torrent_name text,
		imdb text,
		output text,
		size integer
	)`).Error; err != nil {
		t.Fatal(err)
	}

	const season = "Series/Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV"
	inserts := []string{
		`INSERT INTO torrents (hash, file_index, torrent_name, size) VALUES ('bolt', 0, 'bolt_supercao.mkv', 539402598)`,
		`INSERT INTO torrents (hash, file_index, torrent_name, size) VALUES ('pack', 7, '` + season + `/The.Big.Bang.Theory.S01E02.mkv', 500)`,
		`INSERT INTO torrents (hash, file_index, torrent_name, size) VALUES ('pack', 3, '` + season + `/The.Big.Bang.Theory.S01E01.mkv', 100)`,
	}
	for _, statement := range inserts {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := database.ApplyOne(db, 3); err != nil {
		t.Fatal(err)
	}
	if columnExists(db, "torrents", "torrent_name") {
		t.Fatal("torrent_name still exists")
	}

	var torrents []persistence.TorrentModel
	if err := db.Order("hash").Find(&torrents).Error; err != nil {
		t.Fatal(err)
	}
	if len(torrents) != 2 || torrents[0].Hash != "bolt" || torrents[0].Name != "bolt_supercao.mkv" || torrents[0].Size != 539402598 {
		t.Fatalf("torrents %+v", torrents)
	}
	if torrents[1].Hash != "pack" || torrents[1].Size != 600 || torrents[1].Name != "Big Bang - A Teoria 2006 - 1ª Temporada Completa (720p) Acesse o ORIGINAL WWW.BLUDV.TV" {
		t.Fatalf("pack %+v", torrents[1])
	}

	var movies []persistence.MovieModel
	if err := db.Find(&movies).Error; err != nil {
		t.Fatal(err)
	}
	if len(movies) != 1 || movies[0].TorrentID != torrents[0].ID || movies[0].FileIndex != 0 || movies[0].Size != 539402598 {
		t.Fatalf("movies %+v", movies)
	}

	var series []persistence.SeriesModel
	if err := db.Find(&series).Error; err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 || series[0].TorrentID != torrents[1].ID {
		t.Fatalf("series %+v", series)
	}
	var episodes []persistence.EpisodeModel
	if err := db.Where("series_id = ?", series[0].ID).Order("file_index").Find(&episodes).Error; err != nil {
		t.Fatal(err)
	}
	if len(episodes) != 2 || episodes[0].FileIndex != 3 || episodes[0].Name != "The.Big.Bang.Theory.S01E01.mkv" || episodes[1].FileIndex != 7 {
		t.Fatalf("episodes %+v", episodes)
	}
}
