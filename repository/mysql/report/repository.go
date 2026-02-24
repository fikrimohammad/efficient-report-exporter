package report

import (
	"github.com/jmoiron/sqlx"

	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

type repo struct {
	db *sqlx.DB
}

func New(db *sqlx.DB) repository.ReportMySQL {
	return &repo{db}
}

const (
	selectReportQuery = `
		SELECT
			id,
			shop_id,
			order_id,
			order_creation_time,
			order_payment_time,
			order_settlement_time,
			fee_id,
			details,
			creation_time,
			update_time
		FROM
		    report
	`
)
