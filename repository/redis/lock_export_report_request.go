package redis

import (
	"context"

	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/repository"
	"github.com/fikrimohammad/go-dev-sdk/errs"
)

func (r *repo) LockExportReportRequest(ctx context.Context, params repository.LockExportReportRequest) (string, error) {
	key := exportReportRequestKey(params.RequestID)
	token := newLockToken()
	ok, err := r.redisCli.SetNX(ctx, key, token, params.TTL).Result()
	if err != nil {
		err = errs.Wrap(apperrors.CacheInternal, "set request lock key", err)
		return "", err
	}

	if !ok {
		return "", errs.New(apperrors.Conflict, "export report request is already being processed")
	}

	return token, nil
}
