package loader

import (
	"context"
	"os"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/go-dev-sdk/confloader"
)

type DynamicConfig struct {
	QueryLimitPerPage confloader.Getter[int]               `conf:"folder=process_export_report,key=query_limit_per_page"`
	ReportLineWorkers confloader.Getter[int]               `conf:"folder=process_export_report,key=report_line_workers"`
	ReportCSVWorkers  confloader.Getter[int]               `conf:"folder=process_export_report,key=report_csv_workers"`
	RequestLockTTL    confloader.Getter[time.Duration]     `conf:"folder=process_export_report,key=request_lock_ttl"`
	ProcessLockTTL    confloader.Getter[time.Duration]     `conf:"folder=process_export_report,key=process_lock_ttl"`
	CSVWriteBufSize   confloader.Getter[int]               `conf:"folder=process_export_report,key=csv_write_buf_size"`
	HandlerTimeouts   confloader.Getter[map[string]string] `conf:"folder=api_handler,key=timeouts"`
}

func LoadDynamicConfig(ctx context.Context, appName string, cfg confloader.Config, userName, password string) (*confloader.Loader[DynamicConfig], error) {
	c := buildDynamicLoaderConfig(appName, cfg, userName, password)
	loader, err := confloader.New[DynamicConfig](ctx, c)
	if err != nil {
		return nil, err
	}

	return loader, nil
}

func buildDynamicLoaderConfig(appName string, cfg confloader.Config, userName, password string) confloader.Config {
	if cfg.Provider == "" {
		cfg.Provider = confloader.ProviderEtcd
	}

	if cfg.Namespace == "" {
		cfg.Namespace = appName
	}

	if cfg.AuthClientID == "" {
		if userName != "" {
			cfg.AuthClientID = userName
		} else {
			cfg.AuthClientID = os.Getenv("EFFICIENT_REPORT_EXPORTER_DYNAMIC_CONFIG_CLIENT_ID")
		}
	}

	if cfg.AuthClientSecret == "" {
		if password != "" {
			cfg.AuthClientSecret = password
		} else {
			cfg.AuthClientSecret = os.Getenv("EFFICIENT_REPORT_EXPORTER_DYNAMIC_CONFIG_CLIENT_SECRET")
		}
	}

	if cfg.Environment == "" {
		cfg.Environment = constant.DefaultEnv
	}

	return cfg
}
