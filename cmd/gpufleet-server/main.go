package main

import (
	"flag"
	"log"
	"os"
	"strconv"
	"time"

	"gpufleet/internal/server"
)

func main() {
	var cfg server.Config
	var minFreeMB int
	var retentionDays int
	flag.StringVar(&cfg.Addr, "addr", env("GPUFLEET_ADDR", "127.0.0.1:8080"), "listen address")
	flag.StringVar(&cfg.DataDir, "data-dir", env("GPUFLEET_DATA_DIR", "data"), "runtime data directory")
	flag.StringVar(&cfg.BootstrapDeviceID, "bootstrap-device-id", env("GPUFLEET_BOOTSTRAP_DEVICE_ID", "local-dev"), "initial device id")
	flag.StringVar(&cfg.BootstrapSecret, "bootstrap-secret", env("GPUFLEET_BOOTSTRAP_SECRET", "local-dev-secret"), "initial device secret")
	flag.StringVar(&cfg.AdminPassword, "admin-password", env("GPUFLEET_ADMIN_PASSWORD", ""), "initial admin password")
	flag.StringVar(&cfg.WebDir, "web-dir", env("GPUFLEET_WEB_DIR", "web/dist"), "web dashboard build directory")
	flag.IntVar(&minFreeMB, "min-free-mb", envInt("GPUFLEET_MIN_FREE_MB", 800), "minimum free disk space before rejecting metrics")
	flag.IntVar(&retentionDays, "retention-days", envInt("GPUFLEET_RETENTION_DAYS", 30), "compressed metric retention days")
	flag.Parse()

	cfg.MinFreeBytes = uint64(minFreeMB) * 1024 * 1024
	cfg.Retention = time.Duration(retentionDays) * 24 * time.Hour

	logger := log.New(os.Stdout, "gpufleet-server ", log.LstdFlags|log.Lmsgprefix)
	app, generatedPassword, err := server.NewApp(cfg, logger)
	if err != nil {
		logger.Fatal(err)
	}
	if generatedPassword != "" {
		logger.Printf("generated admin password: %s", generatedPassword)
	}
	logger.Printf("listening on http://%s", cfg.Addr)
	logger.Printf("data dir: %s, min free: %d MiB, retention: %d days", cfg.DataDir, minFreeMB, retentionDays)
	if err := app.ListenAndServe(); err != nil {
		logger.Fatal(err)
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	if value, err := strconv.Atoi(raw); err == nil {
		return value
	}
	return fallback
}
