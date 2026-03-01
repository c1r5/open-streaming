package sqlite

import (
	"github.com/c1r5/open-streaming/src/core/database"
	"github.com/c1r5/open-streaming/src/modules/torrent"
	"gorm.io/gorm"
)

func init() {
	database.RegisterMigration(database.Migration{
		Version:     1,
		Description: "Create torrents table",
		Up: func(db *gorm.DB) error {
			return db.AutoMigrate(&torrent.TorrentModel{})
		},
		Down: func(db *gorm.DB) error {
			return db.Migrator().DropTable(&torrent.TorrentModel{})
		},
	})
}
