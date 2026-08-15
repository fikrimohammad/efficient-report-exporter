package constant

import "time"

type MQTopic string

const (
	MQTopicReporting MQTopic = "reporting"
)

type MQMsgTag string

const (
	MQMsgTagExportReportProcess MQMsgTag = "export_report_process"
	MQMsgTagExportReportDone    MQMsgTag = "export_report_done"
)

type ExportReportJobStatus string

const (
	ExportReportJobStatusProcessing ExportReportJobStatus = "processing"
	ExportReportJobStatusSuccess    ExportReportJobStatus = "success"
	ExportReportJobStatusFailed     ExportReportJobStatus = "failed"
)

type RedisKeyPrefix string

const (
	RedisKeyPrefixExportReportRequest RedisKeyPrefix = "export_report_request"
	RedisKeyPrefixExportReportJob     RedisKeyPrefix = "export_report_job"
)

const (
	ReportLineTimeFormat  = "2006-01-02 15:04:05"
	ReportBatchTimeFormat = "20060102150405"
	ReportCSVExtension    = ".csv"
	ReportZipExtension    = ".zip"
	ReportFileBucket      = "reports"
)

const (
	DefaultQueryLimitPerPage       = 1000
	DefaultMaxTimeRangePerBatch    = 2 * time.Hour
	DefaultMaxBatchPipelineWorkers = 8
	DefaultMaxSingleFileRows       = 100_000
)

const (
	DefaultRequestLockTTL  = 5 * time.Second
	DefaultProcessLockTTL  = 1 * time.Minute
	DefaultCSVWriteBufSize = 1024 * 1024
	// DefaultPipeBufferSize bounds the in-memory byte pipe between the
	// CSV/zip producer and the S3 uploader, letting the producer run ahead of
	// the consumer instead of rendezvousing on every write.
	DefaultPipeBufferSize = 4 * 1024 * 1024
)

const (
	QueryLimitOne             = 1
	ContentTypeOctetStream    = "application/octet-stream"
	ContentDispositionPattern = `attachment; filename="%s"`
)

const (
	DefaultListExportReportJobsLimit = 20
	MaxListExportReportJobsLimit     = 100
	PresignedURLDefaultExpiry        = 15 * time.Minute
	MaxExportTimeRange               = 90 * 24 * time.Hour
)

var ReportFileCSVHeaders = []string{
	"Shop ID",
	"Fee ID",
	"Order ID",
	"Order Creation Time",
	"Order Payment Time",
	"Order Settlement Time",
	"Order Detail ID",
	"Product ID",
	"Category ID",
	"Product Price Amount",
	"Promo Amount",
	"Fee Base Amount",
	"Fee Final Amount",
}
