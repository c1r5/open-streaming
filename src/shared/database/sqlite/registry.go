package sqlite

import (
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	"github.com/c1r5/open-streaming/src/shared/database"
	"gorm.io/gorm"
)

func init() {
	database.RegisterMigration(database.Migration{
		Version:     1,
		Description: "Create torrents table",
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&persistence.TorrentModel{})
		},
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&persistence.TorrentModel{})
		},
	})

	database.RegisterMigration(database.Migration{
		Version:     2,
		Description: "Um torrent pode ter uma linha por arquivo de vídeo",
		Up: func(db *gorm.DB) error {
			if columnExists(db, "torrents", "torrent_name") {
				if db.Migrator().HasIndex(&persistence.TorrentModel{}, "idx_torrents_hash") {
					if err := db.Migrator().DropIndex(&persistence.TorrentModel{}, "idx_torrents_hash"); err != nil {
						return err
					}
				}
				return db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_torrent_file ON torrents(hash, file_index)").Error
			}
			return db.AutoMigrate(&persistence.TorrentModel{})
		},
		Down: func(db *gorm.DB) error {
			if db.Migrator().HasIndex(&persistence.TorrentModel{}, "idx_torrent_file") {
				if err := db.Migrator().DropIndex(&persistence.TorrentModel{}, "idx_torrent_file"); err != nil {
					return err
				}
			}
			return db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_torrents_hash ON torrents(hash)").Error
		},
	})

	database.RegisterMigration(database.Migration{
		Version:     3,
		Description: "Separa filmes e séries do torrent",
		Up:          migrateLibrary,
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&persistence.EpisodeModel{}, &persistence.SeriesModel{}, &persistence.MovieModel{})
		},
	})
}

func columnExists(db *gorm.DB, table, column string) bool {
	var count int64
	err := db.Raw(
		"SELECT COUNT(*) FROM pragma_table_info('"+table+"') WHERE name = ?",
		column,
	).Scan(&count).Error
	return err == nil && count > 0
}
