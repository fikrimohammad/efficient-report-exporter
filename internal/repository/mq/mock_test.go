package mq

import (
	"testing"

	"github.com/fikrimohammad/efficient-report-exporter/internal/mock"
	"go.uber.org/mock/gomock"
)

func newMockProducerClient(t *testing.T) *mock.MockProducerClient {
	t.Helper()
	return mock.NewMockProducerClient(gomock.NewController(t))
}
