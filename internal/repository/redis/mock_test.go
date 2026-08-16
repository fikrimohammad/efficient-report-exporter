package redis

import (
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/mock"
	"go.uber.org/mock/gomock"
)

func newMockRedisClient(t *testing.T) *mock.MockRedisClient {
	t.Helper()
	return mock.NewMockRedisClient(gomock.NewController(t))
}
