package loader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fikrimohammad/efficient-report-exporter/common/apiserver"
	"github.com/fikrimohammad/efficient-report-exporter/common/db"
	"github.com/fikrimohammad/efficient-report-exporter/common/observability/metrics"
	"github.com/fikrimohammad/efficient-report-exporter/common/observability/tracer"
	commonredis "github.com/fikrimohammad/efficient-report-exporter/common/redis"
	rocketmqconsumer "github.com/fikrimohammad/efficient-report-exporter/common/rocketmq/consumer"
	rocketmqproducer "github.com/fikrimohammad/efficient-report-exporter/common/rocketmq/producer"
	commons3 "github.com/fikrimohammad/efficient-report-exporter/common/s3"

	"github.com/fikrimohammad/efficient-report-exporter/common/confloader"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
)

// AppConfig is the final merged config from file + secrets.
type AppConfig struct {
	DB          db.Config
	Redis       commonredis.Config
	MQProducers []rocketmqproducer.Config
	MQConsumers []rocketmqconsumer.Config
	S3          commons3.Config
	Metrics     metrics.Config
	Tracer      tracer.Config
	APIServer   apiserver.Config
	Dynamic     *confloader.Loader[DynamicConfig]
}

// Load merges the config file at path (or the CONFIG_PATH override, else the
// APP_ENV-derived default) with the secrets and dynamic config, returning the
// final AppConfig.
func Load(ctx context.Context, path string) (*AppConfig, error) {
	if path == "" {
		path = resolvePath()
	}

	fileCfg, err := LoadFile(path)
	if err != nil {
		return nil, err
	}

	secretLoader, err := LoadSecret(ctx, constant.AppName, fileCfg.SecretLoader)
	if err != nil {
		return nil, err
	}
	secrets := secretLoader.Data()
	_ = secretLoader.Stop()

	dynamicConfigLoader, err := LoadDynamicConfig(ctx, constant.AppName, fileCfg.DynamicLoader,
		secrets.DynamicConfigUserName.GetWithDefault(ctx, ""),
		secrets.DynamicConfigPassword.GetWithDefault(ctx, ""),
	)
	if err != nil {
		return nil, err
	}

	apiAddr := fileCfg.APIServer.Addr
	if apiAddr == "" {
		apiAddr = ":3000"
	}
	shutdownTimeout := fileCfg.APIServer.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = constant.ServerShutdownTimeout
	}

	dbCfg := fileCfg.DB
	dbCfg.Username = secrets.DBUserName.GetWithDefault(ctx, "")
	dbCfg.Password = secrets.DBPassword.GetWithDefault(ctx, "")

	redisCfg := fileCfg.Redis
	redisCfg.Username = secrets.RedisUserName.GetWithDefault(ctx, "")
	redisCfg.Password = secrets.RedisPassword.GetWithDefault(ctx, "")

	s3Cfg := fileCfg.S3
	s3Cfg.AccessKeyID = secrets.S3AccessKeyID.GetWithDefault(ctx, "")
	s3Cfg.SecretAccessKey = secrets.S3SecretAccessKey.GetWithDefault(ctx, "")

	return &AppConfig{
		DB:          dbCfg,
		Redis:       redisCfg,
		S3:          s3Cfg,
		MQProducers: fileCfg.MQProducers,
		MQConsumers: fileCfg.MQConsumers,
		Metrics:     fileCfg.Metrics,
		Tracer:      fileCfg.Tracer,
		APIServer: apiserver.Config{
			Addr:            apiAddr,
			ReadTimeout:     fileCfg.APIServer.ReadTimeout,
			WriteTimeout:    fileCfg.APIServer.WriteTimeout,
			ShutdownTimeout: shutdownTimeout,
		},
		Dynamic: dynamicConfigLoader,
	}, nil
}

// resolvePath returns the config file path for the current environment,
// derived from APP_ENV.
func resolvePath() string {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = constant.DefaultEnv
	}
	return filepath.Join("config", fmt.Sprintf(constant.ConfigFileFormat, env))
}
