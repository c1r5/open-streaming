package database

import (
	"database/sql"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

var (
	instance *gorm.DB
	once     sync.Once
	err      error
)

func Connect(d gorm.Dialector) error {
	once.Do(func() {
		instance, err = gorm.Open(d, &gorm.Config{})
		if err != nil {
			return
		}

		var sqlDB *sql.DB
		sqlDB, err = instance.DB()
		if err != nil {
			return
		}

		sqlDB.SetMaxOpenConns(1)

		for _, pragma := range []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA busy_timeout=5000",
			"PRAGMA synchronous=NORMAL",
		} {
			if _, err = sqlDB.Exec(pragma); err != nil {
				return
			}
		}

		err = Apply(instance)
	})

	if err != nil {
		return fmt.Errorf("database_connection_error: %s", err.Error())
	}

	return nil
}

func GetInstance() *gorm.DB {
	return instance
}

func Close() error {
	if instance != nil {
		db, err := instance.DB()
		if err != nil {
			return fmt.Errorf("database_close_error: %v", err)
		}

		if err := db.Close(); err != nil {
			return fmt.Errorf("database_close_error: %v", err)
		}
	}

	return nil
}
