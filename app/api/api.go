package api

import (
	"context"
	"time"

	"github.com/fikrimohammad/efficient-report-exporter/app"
	"github.com/fikrimohammad/efficient-report-exporter/constant"
	apihandler "github.com/fikrimohammad/efficient-report-exporter/handler/api"
	"github.com/fikrimohammad/go-dev-sdk/apiserver"
	"github.com/fikrimohammad/go-dev-sdk/observability/metrics"
	"github.com/fikrimohammad/go-dev-sdk/observability/tracer"
)

type Server struct {
	handler       *apihandler.Handler
	hz            *apiserver.Server
	metricsClient metrics.Client
	tracerClient  tracer.Client
}

func New(src *app.Resource) (*Server, error) {
	handler, err := apihandler.New(src.ReportUseCase)
	if err != nil {
		return nil, err
	}
	return &Server{
		handler:       handler,
		metricsClient: src.MetricsClient,
		tracerClient:  src.TracerClient,
	}, nil
}

func (s *Server) Start(cfg apiserver.Config) error {
	srv, err := apiserver.New(cfg, s.metricsClient, s.tracerClient)
	if err != nil {
		return err
	}
	s.hz = srv

	hz := s.hz.Hertz()
	hz.POST(constant.RouteExportReport, s.handler.RequestExportReport)
	hz.GET(constant.RouteExportReportJob, s.handler.GetExportReportJob)
	hz.GET(constant.RouteExportReportJobs, s.handler.ListExportReportJobs)

	return s.hz.Run()
}

func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.hz.Shutdown(ctx)
}
