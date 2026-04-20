// ptd is the portfolio-tracker HTTP server.
//
// It opens the SQLite database, applies migrations, mounts the v1 API
// plus the embedded Preact frontend, and runs a periodic price/FX refresh
// loop in the background. Flags override environment variables when both
// are set.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/api"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/prices"
	"github.com/mtzanidakis/portfolio-tracker/internal/version"
	"github.com/mtzanidakis/portfolio-tracker/internal/web"
)

func main() {
	os.Exit(run())
}

func run() int {
	var (
		addr         = flag.String("addr", envOr("PT_ADDR", ":8080"), "HTTP listen address")
		dbPath       = flag.String("db", envOr("PT_DB", "./data/pt.db"), "SQLite database path")
		refreshEvery = flag.Duration("refresh", envDur("PT_PRICE_REFRESH_INTERVAL", 15*time.Minute), "price/fx refresh interval")
		cgKey        = flag.String("coingecko-api-key", os.Getenv("PT_COINGECKO_API_KEY"), "optional CoinGecko Demo API key")
		showVersion  = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("ptd", version.Version)
		return 0
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("portfolio-tracker starting",
		"version", version.Version, "addr", *addr, "db", *dbPath, "refresh", *refreshEvery)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	conn, err := db.Open(ctx, *dbPath)
	if err != nil {
		logger.Error("open db", "err", err)
		return 1
	}
	defer func() { _ = conn.Close() }()
	if err := conn.Migrate(ctx); err != nil {
		logger.Error("migrate", "err", err)
		return 1
	}

	// Background price/FX refresh.
	svc := prices.New(conn, logger, *cgKey)
	go svc.Run(ctx, *refreshEvery)

	// HTTP mux: API + embedded frontend.
	mux := api.NewRouter(conn)
	mux.Handle("/", web.DefaultHandler())

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown.
	go func() {
		<-ctx.Done()
		logger.Info("shutting down")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		_ = srv.Shutdown(shutCtx)
	}()

	logger.Info("listening", "addr", *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server error", "err", err)
		return 1
	}
	return 0
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDur(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	// Accept raw-seconds ints as well for compose-file ergonomics.
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	return def
}
