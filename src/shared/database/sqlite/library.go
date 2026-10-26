package sqlite

import (
	"path"
	"strings"
	"time"

	"github.com/c1r5/open-streaming/src/modules/torrent_engine/domain"
	"github.com/c1r5/open-streaming/src/modules/torrent_engine/infrastructure/persistence"
	"gorm.io/gorm"
)

type storedFile struct {
	CreatedAt   time.Time
	Hash        string
	FileIndex   int
	TorrentName string
	IMDB        *string
	Size        int64
	DeletedAt   *time.Time
}

type savedLibrary struct {
	id    uint
	files []domain.VideoMeta
	imdb  *string
	when  time.Time
}

func migrateLibrary(db *gorm.DB) error {
	var saved []savedLibrary
	if columnExists(db, "torrents", "torrent_name") {
		var err error
		saved, err = copyLibrary(db)
		if err != nil {
			return err
		}
	}
	if err := db.AutoMigrate(
		&persistence.TorrentModel{},
		&persistence.MovieModel{},
		&persistence.SeriesModel{},
		&persistence.EpisodeModel{},
	); err != nil {
		return err
	}
	return writeLibrary(db, saved)
}

func copyLibrary(db *gorm.DB) ([]savedLibrary, error) {
	var rows []storedFile
	if err := db.Table("torrents").Find(&rows).Error; err != nil {
		return nil, err
	}

	type group struct {
		files []storedFile
	}
	order := make([]string, 0)
	grouped := map[string]*group{}
	for _, row := range rows {
		if row.DeletedAt != nil {
			continue
		}
		item, ok := grouped[row.Hash]
		if !ok {
			item = &group{}
			grouped[row.Hash] = item
			order = append(order, row.Hash)
		}
		item.files = append(item.files, row)
	}

	if err := db.Exec(`CREATE TABLE torrents_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME,
		hash TEXT,
		name TEXT,
		size INTEGER
	)`).Error; err != nil {
		return nil, err
	}
	if err := db.Exec("CREATE UNIQUE INDEX idx_torrents_hash ON torrents_new(hash)").Error; err != nil {
		return nil, err
	}
	if err := db.Exec("CREATE INDEX idx_torrents_deleted_at ON torrents_new(deleted_at)").Error; err != nil {
		return nil, err
	}

	savedRows := make([]savedLibrary, 0, len(order))
	now := time.Now()
	for _, hash := range order {
		files := grouped[hash].files
		videos := make([]domain.VideoMeta, 0, len(files))
		var imdb *string
		oldest := files[0].CreatedAt
		var total int64
		for _, file := range files {
			videos = append(videos, domain.VideoMeta{
				Index: file.FileIndex,
				Name:  file.TorrentName,
				Size:  file.Size,
			})
			total += file.Size
			if imdb == nil && file.IMDB != nil && *file.IMDB != "" {
				value := *file.IMDB
				imdb = &value
			}
			if file.CreatedAt.Before(oldest) {
				oldest = file.CreatedAt
			}
		}
		domain.SortVideos(videos)
		if err := db.Exec(
			"INSERT INTO torrents_new (created_at, updated_at, hash, name, size) VALUES (?, ?, ?, ?, ?)",
			oldest, now, hash, domain.PackTitle(videos[0].Name), total,
		).Error; err != nil {
			return nil, err
		}
		var id uint
		if err := db.Raw("SELECT id FROM torrents_new WHERE hash = ?", hash).Scan(&id).Error; err != nil {
			return nil, err
		}
		savedRows = append(savedRows, savedLibrary{id: id, files: videos, imdb: imdb, when: oldest})
	}

	if err := db.Exec("DROP TABLE torrents").Error; err != nil {
		return nil, err
	}
	if err := db.Exec("ALTER TABLE torrents_new RENAME TO torrents").Error; err != nil {
		return nil, err
	}
	return savedRows, nil
}

func writeLibrary(db *gorm.DB, savedRows []savedLibrary) error {
	now := time.Now()
	for _, item := range savedRows {
		if domain.IsSeries(item.files) {
			series := persistence.SeriesModel{TorrentID: item.id, IMDB: item.imdb, Model: gorm.Model{CreatedAt: item.when, UpdatedAt: now}}
			if err := db.Create(&series).Error; err != nil {
				return err
			}
			for _, file := range item.files {
				episode := persistence.EpisodeModel{
					SeriesID:  series.ID,
					FileIndex: file.Index,
					Name:      path.Base(strings.ReplaceAll(file.Name, "\\", "/")),
					Size:      file.Size,
					Model:     gorm.Model{CreatedAt: item.when, UpdatedAt: now},
				}
				if err := db.Create(&episode).Error; err != nil {
					return err
				}
			}
			continue
		}
		file := item.files[0]
		movie := persistence.MovieModel{
			TorrentID: item.id,
			FileIndex: file.Index,
			Size:      file.Size,
			IMDB:      item.imdb,
			Model:     gorm.Model{CreatedAt: item.when, UpdatedAt: now},
		}
		if err := db.Create(&movie).Error; err != nil {
			return err
		}
	}
	return nil
}
