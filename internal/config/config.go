package config

import (
	"context"
	"os"

	"github.com/fikrimohammad/efficient-report-exporter/internal/config/loader"
)

type AppConfig = loader.AppConfig
type DynamicConfig = loader.DynamicConfig

// Load merges the config file with the secrets and dynamic config. The file
// path is taken from the CONFIG_PATH env var when set, otherwise it is derived
// from APP_ENV.
func Load(ctx context.Context) (*AppConfig, error) {
	return loader.Load(ctx, os.Getenv("CONFIG_PATH"))
}
