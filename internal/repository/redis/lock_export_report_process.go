package redis

import (
	"context"

	"github.com/fikrimohammad/efficient-report-exporter/internal/constant"
	"github.com/fikrimohammad/efficient-report-exporter/internal/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs/v2"
)

func (r *repo) LockExportReportProcess(ctx context.Context, params repository.LockExportReportProcess) (string, error) {
	key := exportReportProcessKey(params.JobID)
	token := newLockToken()
	ok, err := r.redisCli.SetNX(ctx, key, token, params.TTL).Result()
	if err != nil {
		err = errs.Wrap(constant.CacheInternal, "set process lock key", err)
		return "", err
	}

	if !ok {
		return "", errs.New(constant.Conflict, "export report process is already running")
	}

	return token, nil
}
