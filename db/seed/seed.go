package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	snowflake "github.com/godruoyi/go-snowflake"

	"github.com/fikrimohammad/efficient-report-exporter/internal/config"
)

type feeDetail struct {
	OrderDetailID      int64   `json:"order_detail_id"`
	ProductID          int64   `json:"product_id"`
	CategoryID         int64   `json:"category_id"`
	ProductPriceAmount float64 `json:"product_price_amount"`
	PromoAmount        float64 `json:"promo_amount"`
	FeeBaseAmount      float64 `json:"fee_base_amount"`
	FeeFinalAmount     float64 `json:"fee_final_amount"`
}

type exportExtra struct {
	ErrCode  *int    `json:"err_code,omitempty"`
	ErrMsg   *string `json:"err_msg,omitempty"`
	FileName *string `json:"file_name,omitempty"`
}

var seeders = map[string]func(*sql.DB){
	"report":            seedReports,
	"export_report_job": seedExportReportJobs,
}

var seederOrder = []string{"report", "export_report_job"}

func main() {
	ctx := context.Background()

	cfg, err := config.Load(ctx)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	defer func() { _ = cfg.Dynamic.Stop() }()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&multiStatements=true&charset=utf8mb4",
		cfg.DB.Username, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}

	tables := resolveTables()
	log.Printf("seeding tables: %v", tables)

	for _, name := range tables {
		fn, ok := seeders[name]
		if !ok {
			log.Fatalf("unknown table: %s (available: report, export_report_job)", name)
		}
		log.Printf("--- seeding %s ---", name)
		fn(db)
	}

	log.Println("seed completed successfully")
}

func resolveTables() []string {
	args := os.Args[1:]
	if len(args) == 0 {
		return seederOrder
	}

	seen := make(map[string]bool)
	var tables []string
	for _, a := range args {
		if a == "all" {
			return seederOrder
		}
		if seen[a] {
			continue
		}
		seen[a] = true
		tables = append(tables, a)
	}
	return tables
}

func seedReports(db *sql.DB) {
	const (
		numShops       = 50
		reportsPerShop = 4000
		batchSize      = 1000
		monthsBack     = 6
	)

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("failed to begin transaction for report: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		log.Fatalf("failed to disable FK checks: %v", err)
	}
	if _, err := tx.Exec("TRUNCATE TABLE report"); err != nil {
		log.Fatalf("failed to truncate report: %v", err)
	}
	if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS = 1"); err != nil {
		log.Fatalf("failed to enable FK checks: %v", err)
	}

	now := time.Now().UTC()
	startWindow := now.AddDate(0, -monthsBack, 0)
	windowDuration := now.Sub(startWindow)

	totalRows := numShops * reportsPerShop
	inserted := 0
	nextFeeID := int64(1)
	batch := make([]string, 0, batchSize)
	args := make([]any, 0, batchSize*7)

	for shopNum := 0; shopNum < numShops; shopNum++ {
		shopID := int64(1001 + shopNum)
		orderBase := shopID * 1_000_000

		for o := 0; o < reportsPerShop; o++ {
			orderSettlementTime := startWindow.Add(time.Duration(float64(windowDuration) * rand.Float64()))
			orderPaymentTime := orderSettlementTime.Add(-time.Duration(rand.Intn(72)+1) * time.Hour)
			orderCreationTime := orderPaymentTime.Add(-time.Duration(rand.Intn(24)+1) * time.Hour)

			numDetails := rand.Intn(5) + 1
			details := make([]feeDetail, numDetails)
			for d := 0; d < numDetails; d++ {
				productID := rand.Int63n(5000) + 1
				details[d] = feeDetail{
					OrderDetailID:      int64(o*5 + d + 1),
					ProductID:          productID,
					CategoryID:         (productID % 50) + 1,
					ProductPriceAmount: float64(rand.Intn(990000)+10000) / 100,
					PromoAmount:        float64(rand.Intn(50000)) / 100,
					FeeBaseAmount:      float64(rand.Intn(50000)) / 100,
					FeeFinalAmount:     float64(rand.Intn(50000)) / 100,
				}
			}
			detailsJSON, err := json.Marshal(details)
			if err != nil {
				log.Fatalf("failed to marshal details: %v", err)
			}

			batch = append(batch, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			args = append(args, shopID, orderBase+int64(o)+1, orderCreationTime.UnixMilli(), orderPaymentTime.UnixMilli(), orderSettlementTime.UnixMilli(), nextFeeID, string(detailsJSON), now.UnixMilli(), now.UnixMilli())
			nextFeeID++

			if len(batch) >= batchSize {
				insertReportBatch(tx, batch, args)
				inserted += len(batch)
				log.Printf("  report progress: %d / %d (%.1f%%)", inserted, totalRows, float64(inserted)*100/float64(totalRows))
				batch = batch[:0]
				args = args[:0]
			}
		}
	}

	if len(batch) > 0 {
		insertReportBatch(tx, batch, args)
		inserted += len(batch)
		log.Printf("  report progress: %d / %d (%.1f%%)", inserted, totalRows, float64(inserted)*100/float64(totalRows))
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("failed to commit report seed: %v", err)
	}
	log.Printf("  report: %d rows seeded (atomic)", inserted)
}

func insertReportBatch(tx *sql.Tx, placeholders []string, args []any) {
	query := "INSERT INTO report (shop_id, order_id, order_creation_time, order_payment_time, order_settlement_time, fee_id, details, creation_time, update_time) VALUES "
	query += joinPlaceholders(placeholders)
	if _, err := tx.Exec(query, args...); err != nil {
		log.Fatalf("failed to insert report batch: %v", err)
	}
}

func seedExportReportJobs(db *sql.DB) {
	const (
		batchSize = 100
		totalJobs = 500
	)

	tx, err := db.Begin()
	if err != nil {
		log.Fatalf("failed to begin transaction for export_report_job: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		log.Fatalf("failed to disable FK checks: %v", err)
	}
	if _, err := tx.Exec("TRUNCATE TABLE export_report_job"); err != nil {
		log.Fatalf("failed to truncate export_report_job: %v", err)
	}
	if _, err := tx.Exec("SET FOREIGN_KEY_CHECKS = 1"); err != nil {
		log.Fatalf("failed to enable FK checks: %v", err)
	}

	snowflake.SetMachineID(1)

	statuses := []string{"processing", "success", "failed"}
	shops := make([]int64, 50)
	for i := range shops {
		shops[i] = int64(1001 + i)
	}

	failedCode := 500
	failedMsg := "internal server error"

	batch := make([]string, 0, batchSize)
	args := make([]any, 0, batchSize*9)

	for i := 0; i < totalJobs; i++ {
		jobID, err := snowflake.NextID()
		if err != nil {
			log.Fatalf("failed to generate snowflake ID: %v", err)
		}

		shopID := shops[rand.Intn(len(shops))]
		requestID := int64(10_000_000 + i)
		endTime := time.Now().UnixMilli() - int64(rand.Intn(86400_000*30))
		startTime := endTime - int64(rand.Intn(86400_000*7)+86400_000)
		status := statuses[rand.Intn(len(statuses))]

		var extra exportExtra
		switch status {
		case "success":
			fn := fmt.Sprintf("report_%s.csv", time.Now().Format("20060102"))
			extra = exportExtra{FileName: &fn}
		case "failed":
			extra = exportExtra{ErrCode: &failedCode, ErrMsg: &failedMsg}
		}
		extraJSON, err := json.Marshal(extra)
		if err != nil {
			log.Fatalf("failed to marshal extra: %v", err)
		}

		creationTime := startTime
		var updateTime *int64
		if status != "processing" {
			ut := creationTime + int64(rand.Intn(60000)+1000)
			updateTime = &ut
		}

		if updateTime != nil {
			batch = append(batch, "(?, ?, ?, ?, ?, ?, ?, ?, ?)")
			args = append(args, int64(jobID), requestID, shopID, startTime, endTime, status, string(extraJSON), creationTime, *updateTime)
		} else {
			batch = append(batch, "(?, ?, ?, ?, ?, ?, ?, ?, NULL)")
			args = append(args, int64(jobID), requestID, shopID, startTime, endTime, status, string(extraJSON), creationTime)
		}

		if len(batch) >= batchSize {
			insertJobBatch(tx, batch, args)
			batch = batch[:0]
			args = args[:0]
		}
	}

	if len(batch) > 0 {
		insertJobBatch(tx, batch, args)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("failed to commit export_report_job seed: %v", err)
	}
	log.Printf("  export_report_job: %d rows seeded (atomic)", totalJobs)
}

func insertJobBatch(tx *sql.Tx, placeholders []string, args []any) {
	query := "INSERT INTO export_report_job (id, request_id, shop_id, start_time, end_time, status, extra, creation_time, update_time) VALUES "
	query += joinPlaceholders(placeholders)
	if _, err := tx.Exec(query, args...); err != nil {
		log.Fatalf("failed to insert export_report_job batch: %v", err)
	}
}

func joinPlaceholders(p []string) string {
	n := len(p) - 1
	for _, s := range p {
		n += len(s)
	}
	b := make([]byte, 0, n)
	for i, s := range p {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, s...)
	}
	return string(b)
}
