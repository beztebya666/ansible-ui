// Command api is the ansible-ui control-plane service: REST + WebSocket
// gateway, Postgres persistence, demo seeding and live run orchestration.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nikiv/ansible-ui/internal/api"
	"github.com/nikiv/ansible-ui/internal/config"
	"github.com/nikiv/ansible-ui/internal/store"
)

func main() {
	level := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	cfg := config.Load()
	ctx := context.Background()

	st, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connect failed", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	// Any run still marked running from a previous process is now orphaned.
	if err := st.MarkOrphansFailed(ctx); err != nil {
		log.Warn("orphan cleanup failed", "err", err)
	}

	for _, d := range []string{"projects", "runs"} {
		if err := os.MkdirAll(filepath.Join(cfg.DataDir, d), 0o755); err != nil {
			log.Error("create data dir failed", "dir", d, "err", err)
			os.Exit(1)
		}
	}

	_ = st.PurgeExpiredSessions(ctx)

	srv, err := api.NewServer(cfg, st, log)
	if err != nil {
		log.Error("server init failed", "err", err)
		os.Exit(1)
	}
	if err := srv.Seed(ctx); err != nil {
		log.Warn("seed failed (continuing)", "err", err)
	}

	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	go srv.StartScheduler(schedCtx)

	httpSrv := &http.Server{Addr: cfg.Addr, Handler: srv.Routes()}
	go func() {
		log.Info("api listening", "addr", cfg.Addr, "runner", cfg.RunnerURL, "data", cfg.DataDir)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	log.Info("api stopped")
}
