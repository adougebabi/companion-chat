package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/gateway-go/internal/config"
	"github.com/fluctlight/local-ai-companion/apps/gateway-go/internal/gateway"
)

func main() {
	settings, err := config.FromEnv(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr:              settings.ListenAddress,
		Handler:           gateway.NewServer(settings, &http.Client{Timeout: 5 * time.Second}).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("go public gateway listening on %s", settings.ListenAddress)
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignal, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	case <-shutdownSignal.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("gateway shutdown failed: %v", err)
		}
	}
}
