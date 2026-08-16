package redis

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
	commonredis "github.com/fikrimohammad/go-dev-sdk/redis"
)

type repo struct {
	redisCli commonredis.Client
}

func New(redisCli commonredis.Client) (repository.Redis, error) {
	if redisCli == nil {
		return nil, errs.New(constant.Internal, "redis client is not initialized")
	}

	return &repo{redisCli: redisCli}, nil
}

func exportReportProcessKey(jobID int64) string {
	return fmt.Sprintf("%s:%s", constant.RedisKeyPrefixExportReportJob, strconv.FormatInt(jobID, 10))
}

func exportReportRequestKey(requestID int64) string {
	return fmt.Sprintf("%s:%s", constant.RedisKeyPrefixExportReportRequest, strconv.FormatInt(requestID, 10))
}

// newLockToken returns a random hex token identifying a lock owner.
func newLockToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
