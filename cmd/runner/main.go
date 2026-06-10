// Command runner is the execution microservice: it runs ansible-playbook
// inside a PTY and streams the live output to the api over WebSocket.
//
// When API_URL and RUNNER_ADVERTISE_URL are set, the runner also self-registers
// with the control plane and heartbeats, so it joins the distributed runner pool
// (tagged via RUNNER_TAGS). Without them it behaves as the single built-in runner.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/nikiv/ansible-ui/internal/runner"
)

const runnerVersion = "1.0"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	addr := os.Getenv("RUNNER_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      runner.NewServer(log).Handler(),
		ReadTimeout:  0, // long-lived streaming connections
		WriteTimeout: 0,
	}

	go func() {
		log.Info("runner listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	regCtx, regCancel := context.WithCancel(context.Background())
	go registerLoop(regCtx, log)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	regCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("runner stopped")
}

// registerLoop self-registers with the control plane and heartbeats until ctx is
// done. It is a no-op unless API_URL and RUNNER_ADVERTISE_URL are configured.
func registerLoop(ctx context.Context, log *slog.Logger) {
	apiURL := strings.TrimRight(os.Getenv("API_URL"), "/")
	advertise := os.Getenv("RUNNER_ADVERTISE_URL")
	if apiURL == "" || advertise == "" {
		return
	}
	token := os.Getenv("RUNNER_TOKEN")
	name := os.Getenv("RUNNER_NAME")
	if name == "" {
		name, _ = os.Hostname()
	}
	var tags []string
	for _, t := range strings.Split(os.Getenv("RUNNER_TAGS"), ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	maxConcurrent, _ := strconv.Atoi(os.Getenv("RUNNER_CONCURRENCY")) // 0 = unlimited
	client := &http.Client{Timeout: 10 * time.Second}

	register := func() (string, bool) {
		body, _ := json.Marshal(map[string]any{
			"name": name, "url": advertise, "tags": tags,
			"platform": runtime.GOOS + "/" + runtime.GOARCH, "version": runnerVersion,
			"maxConcurrent": maxConcurrent,
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/api/runners/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Runner-Token", token)
		resp, err := client.Do(req)
		if err != nil {
			return "", false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", false
		}
		var out struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return out.ID, out.ID != ""
	}

	// Register, retrying until the control plane is reachable.
	var id string
	for {
		if rid, ok := register(); ok {
			id = rid
			log.Info("registered with control plane", "api", apiURL, "advertise", advertise, "tags", tags, "id", id)
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}

	beat := func() {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, apiURL+"/api/runners/"+id+"/heartbeat", nil)
		req.Header.Set("X-Runner-Token", token)
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			// Removed from the registry — re-register so we rejoin the pool.
			if rid, ok := register(); ok {
				id = rid
			}
		}
	}
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			beat()
		}
	}
}
