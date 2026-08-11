package loader

import (
	"context"
	"os"

	"github.com/fikrimohammad/go-dev-sdk/confloader"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
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

// DBSecret holds extracted DB secret values.
type DBSecret struct {
	UserName string
	Password string
}

// RedisSecret holds extracted Redis secret values.
type RedisSecret struct {
	UserName string
	Password string
}

// S3Secret holds extracted S3 secret values.
type S3Secret struct {
	AccessKeyID     string
	SecretAccessKey string
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
