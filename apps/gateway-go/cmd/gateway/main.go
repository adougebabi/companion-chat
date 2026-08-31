package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/gateway-go/internal/bff"
	"github.com/fluctlight/local-ai-companion/apps/gateway-go/internal/config"
)

func main() {
	settings, err := config.FromEnv(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}

	server := &http.Server{
		Addr: settings.ListenAddress,
		Handler: bff.New(bff.Options{
			CoreBaseURL:    settings.CoreBaseURL,
			CoreServiceKey: settings.CoreServiceKey,
			TrustedOrigin:  settings.TrustedOrigin,
			SecureCookies:  settings.SecureCookies,
			// Do not use a client-wide timeout: conversation turns are streamed and
			// their lifetime is governed by the request context/connection.
			Client: &http.Client{},
		}).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		// Provider-backed Core turns may legitimately spend several minutes
		// before the first visible token. Keep the connection alive for the
		// ten-minute acceptance budget plus a small transport margin.
		IdleTimeout: 11 * time.Minute,
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
