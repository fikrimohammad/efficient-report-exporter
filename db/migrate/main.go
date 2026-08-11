package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/fikrimohammad/efficient-report-exporter/config/loader"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
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

	dbHost, dbPort, dbName := loadDBConfig()
	dbUser, dbPass := loadDBSecrets(ctx)

	dsn := fmt.Sprintf("mysql://%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&multiStatements=true",
		dbUser, dbPass, dbHost, dbPort, dbName)

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

func loadDBConfig() (host, port, name string) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = constant.DefaultEnv
	}

	fileCfg, err := loader.LoadFile(filepath.Join("config", fmt.Sprintf(constant.ConfigFileFormat, env)))
	if err != nil {
		log.Fatalf("failed to load config file: %v", err)
	}

	if fileCfg.DB.Host != "" {
		host = fileCfg.DB.Host
	}
	if host == "" {
		host = envOrDefault("DB_HOST", "127.0.0.1")
	}

	if fileCfg.DB.Port > 0 {
		port = strconv.Itoa(fileCfg.DB.Port)
	}
	if port == "" {
		port = envOrDefault("DB_PORT", "3306")
	}

	if fileCfg.DB.Database != "" {
		name = fileCfg.DB.Database
	}
	if name == "" {
		name = envOrDefault("DB_NAME", "export_report")
	}

	return
}

func loadDBSecrets(ctx context.Context) (user, pass string) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = constant.DefaultEnv
	}

	fileCfg, err := loader.LoadFile(filepath.Join("config", fmt.Sprintf(constant.ConfigFileFormat, env)))
	if err != nil {
		log.Fatalf("failed to load config file: %v", err)
	}

	secretLoader, err := loader.LoadSecret(ctx, constant.AppName, fileCfg.SecretLoader)
	if err != nil {
		log.Fatalf("failed to load secrets from Infisical: %v", err)
	}
	defer func() {
		if stopErr := secretLoader.Stop(); stopErr != nil {
			log.Printf("warning: secret loader stop error: %v", stopErr)
		}
	}()

	secrets := secretLoader.Data()

	user, err = secrets.DBUserName.Get(ctx)
	if err != nil {
		log.Fatalf("failed to get DB username from secrets: %v", err)
	}

	pass, err = secrets.DBPassword.Get(ctx)
	if err != nil {
		log.Fatalf("failed to get DB password from secrets: %v", err)
	}

	return
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
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
