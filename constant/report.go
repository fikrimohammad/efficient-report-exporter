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
	ReportFileNameFormat     = "report_%s_%s_%s.csv"
	ReportFileNameTimeFormat = "20060102"
	ReportLineTimeFormat     = "2006-01-02 15:04:05"
	ReportFileBucket         = "reports"
)

const (
	DefaultQueryLimitPerPage = 1000
	DefaultReportLineWorkers = 32
	DefaultReportCSVWorkers  = 32
)

const (
	DefaultRequestLockTTL  = 5 * time.Second
	DefaultProcessLockTTL  = 1 * time.Minute
	DefaultCSVWriteBufSize = 1024 * 1024
)

const (
	SingleRowQueryLimit       = 1
	ContentTypeCSV            = "text/csv"
	ContentDispositionPattern = `attachment; filename="%s"`
)

const (
	DefaultListExportReportJobsLimit = 20
	MaxListExportReportJobsLimit     = 100
	PresignedURLDefaultExpiry        = 15 * time.Minute
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
