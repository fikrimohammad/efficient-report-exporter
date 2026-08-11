package constant

import "time"

const (
	AppName               = "efficient-report-exporter"
	DefaultEnv            = "development"
	ConfigFileFormat      = "config.%s.yaml"
	MySQLDriverName       = "mysql"
	ServerShutdownTimeout = 10 * time.Second
	APITimeFormat         = "2006-01-02T15:04:05Z"
)

const (
	SnowflakeMachineIDModulus = 1024
)

var SnowflakeEpoch = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
