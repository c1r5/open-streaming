package database

import "gorm.io/gorm"

type Registry interface {
	Register(m Migration)
	GetMigrations() []Migration
	GetMigration(version int) (*Migration, error)
	IsApplied(db *gorm.DB, version int) (bool, error)
	Apply(db *gorm.DB) error
	ApplyOne(db *gorm.DB, version int) error
	Rollback(db *gorm.DB, version int) error
}
