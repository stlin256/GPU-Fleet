package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"gpufleet/internal/server"
	"gpufleet/internal/version"
)

func main() {
	var cfg server.Config
	var minFreeMB int
	var retentionDays int
	var showVersion bool
	flag.StringVar(&cfg.Addr, "addr", env("GPUFLEET_ADDR", "127.0.0.1:8080"), "listen address")
	flag.StringVar(&cfg.DataDir, "data-dir", env("GPUFLEET_DATA_DIR", "data"), "runtime data directory")
	flag.StringVar(&cfg.BootstrapDeviceID, "bootstrap-device-id", env("GPUFLEET_BOOTSTRAP_DEVICE_ID", ""), "initial device id")
	flag.StringVar(&cfg.BootstrapSecret, "bootstrap-secret", env("GPUFLEET_BOOTSTRAP_SECRET", ""), "initial device secret")
	flag.StringVar(&cfg.AdminPassword, "admin-password", env("GPUFLEET_ADMIN_PASSWORD", ""), "initial admin password")
	flag.StringVar(&cfg.WebDir, "web-dir", env("GPUFLEET_WEB_DIR", "web/dist"), "web dashboard build directory")
	flag.StringVar(&cfg.RepoDir, "repo-dir", env("GPUFLEET_REPO_DIR", "."), "Git repository directory for server self-update checks")
	flag.IntVar(&minFreeMB, "min-free-mb", envInt("GPUFLEET_MIN_FREE_MB", 800), "minimum free disk space before rejecting metrics")
	flag.IntVar(&retentionDays, "retention-days", envInt("GPUFLEET_RETENTION_DAYS", 30), "compressed metric retention days")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()
	if showVersion {
		fmt.Println(version.String())
		return
	}
	cfg.AddrExplicit = os.Getenv("GPUFLEET_ADDR") != ""
	flag.Visit(func(item *flag.Flag) {
		if item.Name == "addr" {
			cfg.AddrExplicit = true
		}
	})

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
	logger.Printf("listening on %s://%s", app.Scheme(), app.Addr())
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
