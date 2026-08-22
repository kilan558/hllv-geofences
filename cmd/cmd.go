package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/floriansw/go-hll-rcon/rconv2"
	"github.com/joho/godotenv"
	"github.com/kilan558/hllv-geofences/data"
	"github.com/kilan558/hllv-geofences/worker"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		// Containers receive the same values through Compose env_file without
		// mounting the dotenv file itself.
		if strings.TrimSpace(os.Getenv("HLL_HOST")) == "" {
			slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})).Warn("load-env", "error", err)
		}
	}

	// Set up logger
	level := slog.LevelInfo
	if _, ok := os.LookupEnv("DEBUG"); ok {
		level = slog.LevelDebug
	}
	communityName := strings.TrimSpace(os.Getenv("COMMUNITY_NAME"))
	if communityName == "" {
		communityName = "HLL: Vietnam"
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})).With(
		"application", "hllv-geofences",
		"community", communityName,
	)
	logger.Info("start")

	// Load configuration
	configPath := "./config.yml"
	if path, ok := os.LookupEnv("CONFIG_PATH"); ok {
		configPath = path
	}
	c, err := data.NewConfig(configPath, logger)
	if err != nil {
		logger.Error("config", "error", err)
		return
	}
	if err := c.ApplyServerEnvironment(); err != nil {
		logger.Error("load-server-environment", "error", err)
		return
	}
	if err := c.Validate(); err != nil {
		logger.Error("validate-config", "error", err)
		return
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize workers
	workers := make([]*worker.Worker, 0, len(c.Servers))
	var wg sync.WaitGroup

	for _, server := range c.Servers {
		pool, err := rconv2.NewHLLVConnectionPool(rconv2.ConnectionPoolOptions{
			Logger:   logger,
			Hostname: server.Host,
			Port:     server.Port,
			Password: server.Password,
		})
		if err != nil {
			logger.Error("create-connection-pool", "server", server.Host, "error", err)
			continue
		}
		w := worker.NewWorker(logger, pool, server)
		workers = append(workers, w)

		wg.Add(1)
		go func(w *worker.Worker) {
			defer wg.Done()
			w.Run(ctx)
		}(w)
	}

	// Channel to aggregate restart signals from all workers
	restartCh := make(chan struct{}, 1)
	for _, w := range workers {
		go func(w *worker.Worker) {
			select {
			case <-ctx.Done():
				return
			case <-w.RestartSignal():
				logger.Info("worker-requested-restart", "server", w.Host())
				select {
				case restartCh <- struct{}{}:
				default:
					// Restart already requested
				}
			}
		}(w)
	}

	// Listen for OS signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1)

	// Variable to track if we should restart
	shouldRestart := false

	// Wait for a terminal signal. SIGUSR1 requests a fresh RCON status record
	// for the Discord panel and does not stop the worker.
waitForShutdown:
	for {
		select {
		case sig := <-sigCh:
			logger.Info("received-os-signal", "signal", sig.String())
			if sig == syscall.SIGUSR1 {
				for _, currentWorker := range workers {
					go func(w *worker.Worker) {
						if err := w.RefreshStatus(ctx); err != nil {
							logger.Error("refresh-session-status", "server", w.Host(), "error", err)
						}
					}(currentWorker)
				}
				continue
			}
			if sig == syscall.SIGHUP {
				shouldRestart = true
				logger.Info("restart-requested-via-sighup")
			}
			break waitForShutdown
		case <-restartCh:
			shouldRestart = true
			logger.Info("restart-requested-by-worker")
			break waitForShutdown
		}
	}

	// Graceful shutdown
	logger.Info("initiating-graceful-shutdown")
	cancel()

	// Wait for workers to finish with timeout
	shutdownComplete := make(chan struct{})
	go func() {
		wg.Wait()
		close(shutdownComplete)
	}()

	select {
	case <-shutdownComplete:
		logger.Info("graceful-shutdown-completed")
	case <-time.After(30 * time.Second):
		logger.Warn("shutdown-timeout-exceeded")
	}

	// Exit with appropriate code
	if shouldRestart {
		logger.Info("exiting-for-restart")
		// Exit with 0 so process supervisor (Docker/systemd) can restart us
		os.Exit(0)
	} else {
		logger.Info("exiting-normally")
		os.Exit(0)
	}
}
