package loader

import (
	"context"
	"os"

	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/go-dev-sdk/confloader"
)

// Secret is used by confloader — must have flat Getter fields.
type Secret struct {
	DBUserName            confloader.Getter[string] `conf:"folder=db,key=username"`
	DBPassword            confloader.Getter[string] `conf:"folder=db,key=password"`
	RedisUserName         confloader.Getter[string] `conf:"folder=redis,key=username"`
	RedisPassword         confloader.Getter[string] `conf:"folder=redis,key=password"`
	S3AccessKeyID         confloader.Getter[string] `conf:"folder=s3,key=access_key_id"`
	S3SecretAccessKey     confloader.Getter[string] `conf:"folder=s3,key=secret_access_key"`
	DynamicConfigUserName confloader.Getter[string] `conf:"folder=dynamic_config,key=username"`
	DynamicConfigPassword confloader.Getter[string] `conf:"folder=dynamic_config,key=password"`
}

func LoadSecret(ctx context.Context, appName string, cfg confloader.Config) (*confloader.Loader[Secret], error) {
	c := buildSecretLoaderConfig(appName, cfg)
	loader, err := confloader.New[Secret](ctx, c)
	if err != nil {
		return nil, err
	}

	return loader, nil
}

func buildSecretLoaderConfig(appName string, cfg confloader.Config) confloader.Config {
	if cfg.Provider == "" {
		cfg.Provider = confloader.ProviderInfisical
	}

	if cfg.Namespace == "" {
		cfg.Namespace = appName
	}

	if cfg.AuthClientID == "" {
		cfg.AuthClientID = os.Getenv("EFFICIENT_REPORT_EXPORTER_SECRET_CLIENT_ID")
	}

	if cfg.AuthClientSecret == "" {
		cfg.AuthClientSecret = os.Getenv("EFFICIENT_REPORT_EXPORTER_SECRET_CLIENT_SECRET")
	}

	if cfg.Environment == "" {
		cfg.Environment = constant.DefaultEnv
	}

	return cfg
}
