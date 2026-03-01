package database

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
)

type MigrationRegistry struct {
	migrations []Migration
}

func NewMigrationRegistry() *MigrationRegistry {
	return &MigrationRegistry{
		migrations: make([]Migration, 0),
	}
}

func (r *MigrationRegistry) Register(m Migration) {
	r.migrations = append(r.migrations, m)
}

func (r *MigrationRegistry) GetMigrations() []Migration {
	sorted := make([]Migration, len(r.migrations))
	copy(sorted, r.migrations)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	return sorted
}

func (r *MigrationRegistry) GetMigration(version int) (*Migration, error) {
	for _, m := range r.migrations {
		if m.Version == version {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("migration version %d not found", version)
}

func (r *MigrationRegistry) IsApplied(db *gorm.DB, version int) (bool, error) {
	var record MigrationRecord
	result := db.First(&record, version)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return result.Error == nil, result.Error
}

func (r *MigrationRegistry) Apply(db *gorm.DB) error {
	if err := db.AutoMigrate(&MigrationRecord{}); err != nil {
		return err
	}

	for _, m := range r.GetMigrations() {
		applied, err := r.IsApplied(db, m.Version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		if err := m.Up(db); err != nil {
			return fmt.Errorf("migration %d failed: %w", m.Version, err)
		}

		if err := db.Create(&MigrationRecord{Version: m.Version, AppliedAt: time.Now()}).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *MigrationRegistry) ApplyOne(db *gorm.DB, version int) error {
	if err := db.AutoMigrate(&MigrationRecord{}); err != nil {
		return err
	}

	m, err := r.GetMigration(version)
	if err != nil {
		return err
	}

	applied, err := r.IsApplied(db, version)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	if err := m.Up(db); err != nil {
		return fmt.Errorf("migration %d failed: %w", version, err)
	}

	return db.Create(&MigrationRecord{Version: version, AppliedAt: time.Now()}).Error
}

func (r *MigrationRegistry) Rollback(db *gorm.DB, version int) error {
	if err := db.AutoMigrate(&MigrationRecord{}); err != nil {
		return err
	}

	m, err := r.GetMigration(version)
	if err != nil {
		return err
	}

	applied, err := r.IsApplied(db, version)
	if err != nil {
		return err
	}
	if !applied {
		return fmt.Errorf("migration %d not applied", version)
	}

	if m.Down == nil {
		return fmt.Errorf("migration %d has no rollback", version)
	}

	if err := m.Down(db); err != nil {
		return fmt.Errorf("rollback %d failed: %w", version, err)
	}

	return db.Delete(&MigrationRecord{}, version).Error
}

var _ Registry = (*MigrationRegistry)(nil)

var registry = NewMigrationRegistry()

func RegisterMigration(m Migration) {
	registry.Register(m)
}

func Registered() []Migration {
	return registry.GetMigrations()
}

func Apply(db *gorm.DB) error {
	return registry.Apply(db)
}

func ApplyOne(db *gorm.DB, version int) error {
	return registry.ApplyOne(db, version)
}

func IsApplied(db *gorm.DB, version int) (bool, error) {
	return registry.IsApplied(db, version)
}

func Rollback(db *gorm.DB, version int) error {
	return registry.Rollback(db, version)
}
