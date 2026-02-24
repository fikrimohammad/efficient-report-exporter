package main

import (
	"fmt"
	"log"
	"time"

	reporthandler "github.com/fikrimohammad/efficient-report-exporter/handler/report"
	reportmysqlrepository "github.com/fikrimohammad/efficient-report-exporter/repository/mysql/report"
	reportusecase "github.com/fikrimohammad/efficient-report-exporter/usecase/report"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/jmoiron/sqlx"
)

func main() {
	cfg, err := initConfig()
	if err != nil {
		log.Fatalf("failed to init config: %v", err)
	}

	db, err := initDB(cfg)
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	var (
		reportMySQLRepository = reportmysqlrepository.New(db)
		reportUseCase         = reportusecase.New(reportMySQLRepository)
		reportHandler         = reporthandler.New(reportUseCase)
	)

	app := fiber.New(fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Post("/v1/reports/export", reportHandler.ExportReport)

	log.Fatal(app.Listen(":3000"))
}

func initDB(cfg *config) (*sqlx.DB, error) {
	dbDSNMySQL := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true",
		cfg.db.UserName,
		cfg.db.Password,
		cfg.db.Host,
		cfg.db.Port,
		cfg.db.Name,
	)

	db, err := sqlx.Open("mysql", dbDSNMySQL)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
