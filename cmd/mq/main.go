package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fikrimohammad/efficient-report-exporter/app"
	appmq "github.com/fikrimohammad/efficient-report-exporter/app/mq"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	resource, err := app.NewResource()
	if err != nil {
		log.Fatalf("failed to initialize resources: %v", err)
	}
	defer func() {
		if err := resource.Close(); err != nil {
			log.Printf("failed to close resources: %v", err)
		}
	}()

	mqConsumer, err := appmq.New(resource)
	if err != nil {
		log.Fatalf("failed to initialize mq consumer: %v", err)
	}

	if err := mqConsumer.Start(); err != nil {
		log.Fatalf("failed to start mq consumer: %v", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	if err := mqConsumer.Shutdown(); err != nil {
		log.Printf("failed to shutdown mq consumer: %v", err)
	}
}
