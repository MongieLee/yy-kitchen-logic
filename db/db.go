package db

import (
	"log"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Init opens the database at path, runs AutoMigrate on the supplied models,
// and retries once after recreating the file if migration fails. This
// keeps dev simple when the schema changes (e.g. adding a non-null column
// to a SQLite table). :memory: databases are never recreated.
func Init(path string, models ...any) *gorm.DB {
	inMemory := path == ":memory:"

	open := func() *gorm.DB {
		conn, err := gorm.Open(sqlite.Open(path), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			log.Fatalf("failed to connect to database: %v", err)
		}
		return conn
	}

	migrate := func(conn *gorm.DB) error {
		return conn.AutoMigrate(models...)
	}

	conn := open()
	if err := migrate(conn); err != nil {
		if inMemory {
			log.Fatalf("auto migrate failed: %v", err)
		}
		log.Printf("auto migrate failed (%v); recreating db file and retrying", err)
		sqlDB, _ := conn.DB()
		_ = sqlDB.Close()
		if rmErr := os.Remove(path); rmErr != nil {
			log.Fatalf("failed to remove old db file: %v", rmErr)
		}
		conn = open()
		if err := migrate(conn); err != nil {
			log.Fatalf("auto migrate retry failed: %v", err)
		}
	}

	return conn
}
