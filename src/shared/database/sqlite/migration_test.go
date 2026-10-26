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

	second := persistence.TorrentModel{Hash: "abc", FileIndex: 3, TorrentName: "b.mkv", Size: 2}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("second file of the same torrent: %v", err)
	}
	duplicate := persistence.TorrentModel{Hash: "abc", FileIndex: 3, TorrentName: "again.mkv", Size: 2}
	if err := db.Create(&duplicate).Error; err == nil {
		t.Fatal("duplicate hash and file index was stored")
	}
}
