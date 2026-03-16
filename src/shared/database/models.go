package database

import (
	"time"

	"gorm.io/gorm"
)

type Migration struct {
	Version     int
	Description string
	Up          func(*gorm.DB) error
	Down        func(*gorm.DB) error
}

type MigrationRecord struct {
	Version   int `gorm:"primaryKey"`
	AppliedAt time.Time
}
