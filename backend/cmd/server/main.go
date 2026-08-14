package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"hl7-message-translator/backend/api"
	"hl7-message-translator/backend/metrics"
	"hl7-message-translator/backend/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://hl7:hl7@localhost:5432/hl7_message_translator?sslmode=disable"
	}
	pgStore, err := store.NewPostgresStore(databaseURL)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pgStore.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := pgStore.Ping(pingCtx); err != nil {
		logger.Warn("database not reachable at startup; /ready will report unhealthy until it recovers", "error", err)
	}
	cancel()

	registry := prometheus.NewRegistry()
	m := metrics.New(registry)
	handler := api.NewHandler(pgStore, m, logger)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(api.RequestLogger(logger))

	handler.Routes(r)
	r.Get("/healthz", handler.Health) // kept as an alias of /health for compatibility
	r.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	logger.Info("listening", "addr", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
