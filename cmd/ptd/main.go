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
	"strings"
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
		addr         = flag.String("addr", envOr("PT_ADDR", ":8082"), "HTTP listen address")
		dbPath       = flag.String("db", envOr("PT_DB", "./data/pt.db"), "SQLite database path")
		refreshEvery = flag.Duration("refresh", envDur("PT_PRICE_REFRESH_INTERVAL", 15*time.Minute), "price/fx refresh interval")
		sessionLife  = flag.String("session-lifetime", envOr("PT_SESSION_LIFETIME", "30d"),
			"browser session lifetime (Go duration with optional 'd' days suffix, e.g. 30d, 720h)")
		cgKey       = flag.String("coingecko-api-key", os.Getenv("PT_COINGECKO_API_KEY"), "optional CoinGecko Demo API key")
		showVersion = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Println("ptd", version.Version)
		return 0
	}

	lifetime, err := parseLifetime(*sessionLife)
	if err != nil || lifetime <= 0 {
		fmt.Fprintf(os.Stderr, "ptd: invalid --session-lifetime %q\n", *sessionLife)
		return 2
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger.Info("portfolio-tracker starting",
		"version", version.Version, "addr", *addr, "db", *dbPath,
		"refresh", *refreshEvery, "session_lifetime", lifetime)

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

	// Background session-expiry sweep.
	go runSessionPurge(ctx, conn, logger)

	// HTTP mux: API + embedded frontend. Provider lookups are optional
	// capabilities — only forward them if the concrete provider implements
	// SymbolLookup.
	var yahooLookup, cgLookup prices.SymbolLookup
	if l, ok := svc.Yahoo.(prices.SymbolLookup); ok {
		yahooLookup = l
	}
	if l, ok := svc.CoinGecko.(prices.SymbolLookup); ok {
		cgLookup = l
	}
	mux := api.NewRouter(conn, lifetime,
		api.WithPriceRefresher(svc),
		api.WithAssetLookups(yahooLookup, cgLookup),
	)
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

// parseLifetime accepts Go's time.Duration syntax ("720h", "15m") plus a
// plain "d" suffix for whole days ("30d"). Days are converted to hours.
func parseLifetime(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// runSessionPurge deletes expired sessions every hour until ctx is done.
func runSessionPurge(ctx context.Context, conn *db.DB, logger *slog.Logger) {
	const interval = time.Hour
	if n, err := conn.PurgeExpiredSessions(ctx); err == nil && n > 0 {
		logger.Info("purged expired sessions", "count", n)
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := conn.PurgeExpiredSessions(ctx); err != nil {
				logger.Warn("session purge failed", "err", err)
			} else if n > 0 {
				logger.Info("purged expired sessions", "count", n)
			}
		}
	}
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
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Second
	}
	return def
}
