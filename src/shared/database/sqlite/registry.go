package sqlite

import (
	"github.com/c1r5/open-streaming/src/shared/database"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
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
}
