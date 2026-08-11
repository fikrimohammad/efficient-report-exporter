package redis

import (
	"fmt"
	"strconv"

	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	commonredis "github.com/fikrimohammad/efficient-report-exporter/common/redis"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
)

type repo struct {
	redisCli commonredis.Client
}

func New(redisCli commonredis.Client) (repository.Redis, error) {
	if redisCli == nil {
		return nil, errs.New(errs.Internal, "redis client is not initialized")
	}

	return &repo{redisCli: redisCli}, nil
}

func exportReportProcessKey(jobID int64) string {
	return fmt.Sprintf("%s:%s", constant.RedisKeyPrefixExportReportJob, strconv.FormatInt(jobID, 10))
}

func exportReportRequestKey(requestID int64) string {
	return fmt.Sprintf("%s:%s", constant.RedisKeyPrefixExportReportRequest, strconv.FormatInt(requestID, 10))
}
