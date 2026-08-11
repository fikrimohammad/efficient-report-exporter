package loader

import (
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
	"gopkg.in/yaml.v3"

	"github.com/fikrimohammad/efficient-report-exporter/common/confloader"
)

// FileConfig is the YAML-tagged representation of the config file.
type FileConfig struct {
	DB            db.Config                 `yaml:"db"`
	Redis         commonredis.Config        `yaml:"redis"`
	MQProducers   []rocketmqproducer.Config `yaml:"mq_producers"`
	MQConsumers   []rocketmqconsumer.Config `yaml:"mq_consumers"`
	S3            commons3.Config           `yaml:"s3"`
	Metrics       metrics.Config            `yaml:"metrics"`
	Tracer        tracer.Config             `yaml:"tracer"`
	APIServer     apiserver.Config          `yaml:"api_server"`
	SecretLoader  confloader.Config         `yaml:"secret_loader"`
	DynamicLoader confloader.Config         `yaml:"dynamic_config_loader"`
}

func LoadFile(path string) (*FileConfig, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
