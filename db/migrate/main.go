package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/fikrimohammad/efficient-report-exporter/internal/config"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: %s <up|down|down-all|new> [name]", os.Args[0])
	}
	command := os.Args[1]

	if command == "new" {
		runNew()
		return
	}

	ctx := context.Background()

	cfg, err := config.Load(ctx)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	defer func() { _ = cfg.Dynamic.Stop() }()

	dsn := fmt.Sprintf("mysql://%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&multiStatements=true",
		cfg.DB.Username, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Database)

	m, err := migrate.New("file://db/migrations", dsn)
	if err != nil {
		log.Fatalf("failed to init migrate: %v", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("migrate source close error: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("migrate database close error: %v", dbErr)
		}
	}()

	runMigration(m, command)
}

func runNew() {
	if len(os.Args) < 3 || os.Args[2] == "" {
		log.Fatalf("usage: %s new <migration_name>", os.Args[0])
	}
	name := os.Args[2]
	ts := time.Now().Format("20060102150405")
	base := fmt.Sprintf("db/migrations/%s_%s", ts, name)

	for _, ext := range []string{"up", "down"} {
		path := fmt.Sprintf("%s.%s.sql", base, ext)
		f, err := os.Create(path)
		if err != nil {
			log.Fatalf("failed to create %s: %v", path, err)
		}
		_ = f.Close()
		fmt.Printf("Created: %s\n", path)
	}
}

func runMigration(m *migrate.Migrate, command string) {
	switch command {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate up failed: %v", err)
		}
		log.Println("migration up completed")

	case "down-all":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate down failed: %v", err)
		}
		log.Println("migration down-all completed")

	case "down":
		if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
			log.Fatalf("migrate step -1 failed: %v", err)
		}
		log.Println("migration down (1 step) completed")

	default:
		log.Fatalf("unknown command: %s (use: up, down, down-all)", command)
	}
}
