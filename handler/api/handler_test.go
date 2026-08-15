package api

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/fikrimohammad/efficient-report-exporter/apperrors"
	"github.com/fikrimohammad/efficient-report-exporter/internal/mocks"
	"github.com/fikrimohammad/go-dev-sdk/errs"
	"go.uber.org/mock/gomock"
)

type testServer struct {
	addr   string
	engine *server.Hertz
}

func setupTest(t *testing.T, mock *mocks.MockReport) *testServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	h := &Handler{reportUseCase: mock}
	engine := server.New(server.WithListener(ln), server.WithExitWaitTime(10*time.Millisecond))
	engine.POST("/v1/reports/export", h.RequestExportReport)
	engine.GET("/v1/reports/export/:job_id", h.GetExportReportJob)
	engine.GET("/v1/reports/export", h.ListExportReportJobs)

	go func() { _ = engine.Run() }()

	for i := 0; i < 100; i++ {
		if engine.IsRunning() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	ts := &testServer{addr: ln.Addr().String(), engine: engine}
	t.Cleanup(func() {
		_ = engine.Shutdown(context.Background())
	})

	return ts
}

func newMockReport(t *testing.T) *mocks.MockReport {
	t.Helper()
	return mocks.NewMockReport(gomock.NewController(t))
}

func (ts *testServer) post(t *testing.T, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("POST", "http://"+ts.addr+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func (ts *testServer) get(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", "http://"+ts.addr+path, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func (ts *testServer) readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	_ = resp.Body.Close()
	return string(b)
}

func TestNewHandler_NilUseCase(t *testing.T) {
	_, err := New(nil)
	if err == nil {
		t.Fatal("expected error for nil use case")
	}

	var e *errs.Error
	if !errs.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != apperrors.Internal {
		t.Errorf("expected code Internal, got %v", e.Code())
	}
}
