package repository

import (
	"context"
	"io"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/model"
)

const MaxQueryReportLimit = 1000

type QueryReportFilter struct {
	ShopID                   *int64                `json:"shop_id"`
	OrderSettlementTimeRange *QueryReportTimeRange `json:"order_settlement_time_range"`
	Limit                    int                   `json:"limit"`
	LastReportID             int64                 `json:"last_report_id"`
}

type QueryReportTimeRange struct {
	StartTime *time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
}

type InsertExportReportJobParams struct {
	ShopID    int64                          `json:"shop_id"`
	RequestID int64                          `json:"request_id"`
	StartTime int64                          `json:"start_time"`
	EndTime   int64                          `json:"end_time"`
	Status    constant.ExportReportJobStatus `json:"status"`
	Extra     model.ExportReportJobExtra     `json:"extra"`
}

type QueryExportReportJobFilter struct {
	JobID                 int64 `json:"job_id"`
	RequestID             int64 `json:"request_id"`
	ShopID                int64 `json:"shop_id"`
	Limit                 int   `json:"limit"`
	LastExportReportJobID int64 `json:"last_export_report_job_id"`
}

type UpdateExportReportJobParams struct {
	JobID  int64                          `json:"job_id"`
	Status constant.ExportReportJobStatus `json:"status"`
	Extra  model.ExportReportJobExtra     `json:"extra"`
}

type ReportMySQL interface {
	QueryReport(ctx context.Context, filter QueryReportFilter) ([]*model.Report, error)
	CountReport(ctx context.Context, filter QueryReportFilter) (int64, error)
	QueryExportReportJob(ctx context.Context, params QueryExportReportJobFilter) ([]*model.ExportReportJob, error)
	InsertExportReportJob(ctx context.Context, params InsertExportReportJobParams) (*model.ExportReportJob, error)
	UpdateExportReportJob(ctx context.Context, params UpdateExportReportJobParams) error
}

type ReportMQ interface {
	PublishExportReportProcessMsg(ctx context.Context, params model.ExportReportProcessMessage) error
	PublishExportReportDoneMsg(ctx context.Context, params model.ExportReportDoneMessage) error
}

type LockExportReportRequest struct {
	RequestID int64         `json:"request_id"`
	TTL       time.Duration `json:"ttl"`
}

type UnlockExportReportRequest struct {
	RequestID int64 `json:"request_id"`
}

type LockExportReportProcess struct {
	JobID int64         `json:"job_id"`
	TTL   time.Duration `json:"ttl"`
}

type UnlockExportReportProcess struct {
	JobID int64 `json:"job_id"`
}

type ReportRedis interface {
	LockExportReportProcess(ctx context.Context, params LockExportReportProcess) error
	UnlockExportReportProcess(ctx context.Context, params UnlockExportReportProcess) error
	LockExportReportRequest(ctx context.Context, params LockExportReportRequest) error
	UnlockExportReportRequest(ctx context.Context, params UnlockExportReportRequest) error
}

type UploadReportFileParams struct {
	FileName string        `json:"file_name"`
	FileData io.ReadCloser `json:"file_data"`
}

type GeneratePresignedDownloadURLParams struct {
	FileName  string        `json:"file_name"`
	ExpiresIn time.Duration `json:"expires_in"`
}

type ReportS3 interface {
	UploadReportFile(ctx context.Context, params UploadReportFileParams) error
	GeneratePresignedDownloadURL(ctx context.Context, params GeneratePresignedDownloadURLParams) (string, error)
}
