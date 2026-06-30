package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"alvus/internal/server"

	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the API key rotation proxy server",
	Long:  "Loads configuration, initializes the key pool, and starts the HTTP proxy server.",
	Run: func(cmd *cobra.Command, args []string) {
		local, _ := cmd.Flags().GetBool("local")
		networkOnly, _ := cmd.Flags().GetBool("network-only")
		tag, _ := cmd.Flags().GetString("tag")
		startServer(dashHTML, local, networkOnly, tag)
	},
}

func startServer(dashboardHTML string, isLocal, isNetwork bool, processTag string) {
	// ── Stop channel for graceful shutdown ────────
	stop := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		slog.Info("shutting down")
		close(stop)
	}()

	// ── Host binding ──────────────────────────────
	host := "" // Default (binds to all interfaces)
	if isLocal {
		host = "127.0.0.1"
	} else if isNetwork {
		host = "0.0.0.0"
	}

	// ── Config & Initialization ───────────────────
	cfg, pool := server.LoadConfig()
	server.ApplyLogLevel(cfg.LogLevel)
	state := server.NewServerState(cfg, pool, dashboardHTML, cfg.KeysFile)

	// Initial key pool metric refresh
	state.Metrics().RefreshKeyPoolGauge(pool)

	// ── Background goroutines ─────────────────────
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		server.WatchEnvFile(state, stop)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		server.RefreshKeyPoolMetrics(state, stop)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		server.ActiveHealthCheck(state, stop)
	}()

	// ── HTTP Server ───────────────────────────────
	addr := fmt.Sprintf("%s:%d", host, cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("port in use", "port", cfg.Port, "error", err)
		log.Fatalf("port %d is already in use: %v", cfg.Port, err)
	}

	httpServer := &http.Server{Handler: state.Handler()}

	go func() {
		<-stop
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			slog.Error("shutdown error", "error", err)
		}
	}()

	displayHost := host
	if displayHost == "" {
		displayHost = "0.0.0.0"
	}
	if processTag != "" {
		slog.Info("starting", "tag", processTag, "port", cfg.Port, "keys", len(pool.Keys()), "target", cfg.TargetBase, "genai", cfg.GenaiBase)
	} else {
		slog.Info("starting", "port", cfg.Port, "keys", len(pool.Keys()), "target", cfg.TargetBase, "genai", cfg.GenaiBase)
	}
	if err := httpServer.Serve(listener); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		log.Fatalf("Server error: %v", err)
	}

	wg.Wait()
	slog.Info("server stopped gracefully")
}

func init() {
	rootCmd.AddCommand(startCmd)
}