package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/msomdec/stitch-map-2/internal/handler"
	"github.com/msomdec/stitch-map-2/internal/repository/sqlite"
	"github.com/msomdec/stitch-map-2/internal/service"
)

func main() {
	logOpts := &slog.HandlerOptions{Level: slog.LevelInfo}
	logger := slog.New(slog.NewMultiHandler(
		slog.NewTextHandler(os.Stdout, logOpts),
		slog.NewJSONHandler(os.Stderr, logOpts),
	))
	slog.SetDefault(logger)

	port := envOrDefault("PORT", "8080")
	dbPath := envOrDefault("DATABASE_PATH", "stitch-map.db")
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET environment variable is required")
		os.Exit(1)
	}
	if len(jwtSecret) < 32 {
		slog.Error("JWT_SECRET must be at least 32 characters for HMAC-SHA256 security")
		os.Exit(1)
	}

	// Default to secure cookies; disable only for local development.
	cookieSecure := os.Getenv("COOKIE_SECURE") != "false"

	bcryptCost := 12
	if v := os.Getenv("BCRYPT_COST"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil {
			slog.Error("invalid BCRYPT_COST", "error", err)
			os.Exit(1)
		}
		if parsed < 4 || parsed > 14 {
			slog.Error("BCRYPT_COST must be between 4 and 14", "value", parsed)
			os.Exit(1)
		}
		bcryptCost = parsed
	}

	db, err := sqlite.New(dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(context.Background()); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("database migrations applied")

	authService := service.NewAuthService(db.Users(), jwtSecret, bcryptCost)
	stitchService := service.NewStitchService(db.Stitches())
	patternService := service.NewPatternService(db.Patterns(), db.Stitches())
	sessionService := service.NewWorkSessionService(db.Sessions(), db.Patterns())
	imageService := service.NewImageService(db.PatternImages(), db.FileStore(), db.Patterns())
	shareService := service.NewShareService(db.Shares(), db.Patterns(), db.Users())

	// Seed predefined stitches (idempotent).
	if err := stitchService.SeedPredefined(context.Background()); err != nil {
		slog.Error("failed to seed predefined stitches", "error", err)
		os.Exit(1)
	}
	slog.Info("predefined stitches seeded")

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux, authService, stitchService, patternService, sessionService, imageService, shareService, db.Users(), cookieSecure)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler.SecurityHeaders(mux),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("server starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped")
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
