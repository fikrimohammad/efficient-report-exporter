package mysql

import (
	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/go-dev-sdk/db"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

type repo struct {
	db db.DB
}

func New(db db.DB) (repository.MySQL, error) {
	if db == nil {
		return nil, errs.New(constant.Internal, "database connection is not initialized")
	}

	return &repo{db}, nil
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

	countReportQuery = `
		SELECT COUNT(*)
		FROM
		    report
	`

	selectExportReportJobsQuery = `
		SELECT
			id,
			request_id,
			shop_id,
			start_time,
			end_time,
			status,
			extra,
			creation_time,
			update_time
		FROM
		    export_report_job
	`

	insertExportReportJobQuery = `
		INSERT INTO export_report_job (
			id,
		    request_id,
			shop_id,
			start_time,
			end_time,
			status,
			extra,
			creation_time,
			update_time
		) VALUES (
			:id,
		    :request_id,
			:shop_id,
			:start_time,
			:end_time,
			:status,
			:extra,
			:creation_time,
			:update_time
		)
	`

	updateExportReportJobQuery = `
		UPDATE export_report_job
		SET
			status = :status,
			extra = :extra,
			update_time = :update_time
		WHERE
			id = :id
	`
)
