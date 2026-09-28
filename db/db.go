package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"

	"yy-kitchen-logic/config"
)

// Init opens a bun.DB against MySQL. If the target schema does not yet
// exist it is created (utf8mb4), then bun runs CreateTable IF NOT EXISTS
// for each supplied model.
func Init(cfg config.DatabaseConfig, models ...any) *bun.DB {
	if err := ensureDatabase(cfg); err != nil {
		log.Fatalf("failed to ensure database %q: %v", cfg.Name, err)
	}

	sqldb, err := sql.Open("mysql", dsn(cfg, cfg.Name))
	if err != nil {
		log.Fatalf("failed to open mysql: %v", err)
	}
	sqldb.SetMaxOpenConns(50)
	sqldb.SetMaxIdleConns(10)
	sqldb.SetConnMaxLifetime(30 * time.Minute)

	if err := sqldb.Ping(); err != nil {
		log.Fatalf("failed to ping mysql: %v", err)
	}

	b := bun.NewDB(sqldb, mysqldialect.New())

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	for _, m := range models {
		if _, err := b.NewCreateTable().Model(m).IfNotExists().Exec(ctx); err != nil {
			log.Fatalf("create-table failed: %v", err)
		}
	}

	return b
}

// ensureDatabase connects to MySQL without selecting a schema and issues
// CREATE DATABASE IF NOT EXISTS so first-run works with an empty server.
func ensureDatabase(cfg config.DatabaseConfig) error {
	sqldb, err := sql.Open("mysql", dsn(cfg, ""))
	if err != nil {
		return err
	}
	defer sqldb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", cfg.Name)
	_, err = sqldb.ExecContext(ctx, stmt)
	return err
}

func dsn(cfg config.DatabaseConfig, dbName string) string {
	c := mysql.NewConfig()
	c.User = cfg.User
	c.Passwd = cfg.Password
	c.Net = "tcp"
	c.Addr = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	c.DBName = dbName
	c.ParseTime = true
	c.Loc = time.Local
	c.Params = map[string]string{"charset": "utf8mb4"}
	return c.FormatDSN()
}
