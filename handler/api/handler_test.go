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
	"github.com/fikrimohammad/efficient-report-exporter/common/errs"
	"github.com/fikrimohammad/efficient-report-exporter/usecase"
)

type mockReportUseCase struct {
	requestExportReport  func(ctx context.Context, params usecase.RequestExportReportParams) (*usecase.RequestExportReportResult, error)
	processExportReport  func(ctx context.Context, params usecase.ProcessExportReportParams) error
	getExportReportJob   func(ctx context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error)
	listExportReportJobs func(ctx context.Context, params usecase.ListExportReportJobsParams) (*usecase.ListExportReportJobsResult, error)
}

func (m *mockReportUseCase) RequestExportReport(ctx context.Context, params usecase.RequestExportReportParams) (*usecase.RequestExportReportResult, error) {
	return m.requestExportReport(ctx, params)
}

func (m *mockReportUseCase) ProcessExportReport(ctx context.Context, params usecase.ProcessExportReportParams) error {
	return m.processExportReport(ctx, params)
}

func (m *mockReportUseCase) GetExportReportJob(ctx context.Context, params usecase.GetExportReportJobParams) (*usecase.GetExportReportJobResult, error) {
	return m.getExportReportJob(ctx, params)
}

func (m *mockReportUseCase) ListExportReportJobs(ctx context.Context, params usecase.ListExportReportJobsParams) (*usecase.ListExportReportJobsResult, error) {
	return m.listExportReportJobs(ctx, params)
}

type testServer struct {
	addr   string
	engine *server.Hertz
}

func setupTest(t *testing.T, mock *mockReportUseCase) *testServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}

	h := &Handler{reportUseCase: mock, dynamicConfig: nil}
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
	_, err := New(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil use case")
	}

	var e *errs.Error
	if !errs.As(err, &e) {
		t.Fatalf("expected *errs.Error, got %T", err)
	}
	if e.Code() != errs.Internal {
		t.Errorf("expected code Internal, got %v", e.Code())
	}
}
