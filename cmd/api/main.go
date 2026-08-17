package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fikrimohammad/efficient-report-exporter/internal/app"
	appapi "github.com/fikrimohammad/efficient-report-exporter/internal/app/api"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	defer app.StartPprofServer()()

	resource, err := app.NewResource()
	if err != nil {
		log.Fatalf("failed to initialize resources: %v", err)
	}
	defer func() {
		if err := resource.Close(); err != nil {
			log.Printf("failed to close resources: %v", err)
		}
	}()

	apiServer, err := appapi.New(resource)
	if err != nil {
		log.Fatalf("failed to initialize api server: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- apiServer.Start(resource.Config.APIServer)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-done:
		log.Fatalf("api server exited: %v", err)
	case <-quit:
		if err := apiServer.Shutdown(resource.Config.APIServer.ShutdownTimeout); err != nil {
			log.Printf("failed to shutdown api server: %v", err)
		}
	}
}
