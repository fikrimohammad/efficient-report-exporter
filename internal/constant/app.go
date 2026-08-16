package constant

import "time"

const (
	AppName               = "efficient-report-exporter"
	DefaultEnv            = "development"
	ConfigFileFormat      = "config.%s.yaml"
	ServerShutdownTimeout = 10 * time.Second
	APITimeFormat         = "2006-01-02T15:04:05Z"
)

var SnowflakeEpoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
