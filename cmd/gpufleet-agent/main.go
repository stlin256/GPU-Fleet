package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gpufleet/internal/agent"
)

func main() {
	var serverURL string
	var deviceID string
	var secret string
	var nvidiaSMI string
	var queuePath string
	var intervalSeconds int
	var queueMaxMB int
	var once bool
	var printOnly bool
	var collectProcesses bool
	var gzipBody bool

	flag.StringVar(&serverURL, "server-url", env("GPUFLEET_SERVER_URL", "http://127.0.0.1:8080"), "GPUFleet server URL")
	flag.StringVar(&deviceID, "device-id", env("GPUFLEET_DEVICE_ID", "local-dev"), "device id")
	flag.StringVar(&secret, "secret", env("GPUFLEET_SECRET", "local-dev-secret"), "device secret")
	flag.StringVar(&nvidiaSMI, "nvidia-smi", env("GPUFLEET_NVIDIA_SMI", "nvidia-smi"), "nvidia-smi executable")
	flag.StringVar(&queuePath, "queue-path", env("GPUFLEET_QUEUE_PATH", "agent-queue"), "local offline queue directory")
	flag.IntVar(&intervalSeconds, "interval", envInt("GPUFLEET_INTERVAL", 10), "sample interval seconds")
	flag.IntVar(&queueMaxMB, "queue-max-mb", envInt("GPUFLEET_QUEUE_MAX_MB", 128), "maximum local queue size in MiB")
	flag.BoolVar(&once, "once", false, "collect and upload one sample")
	flag.BoolVar(&printOnly, "print", false, "collect one sample and print JSON without uploading")
	flag.BoolVar(&collectProcesses, "processes", envBool("GPUFLEET_PROCESSES", true), "collect GPU process snapshots")
	flag.BoolVar(&gzipBody, "gzip", true, "gzip request body")
	flag.Parse()

	if printOnly {
		sample, err := agent.NewCollector(nvidiaSMI, 5*time.Second).Collect(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "collector warning: %v\n", err)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(sample); err != nil {
			fmt.Fprintf(os.Stderr, "encode sample: %v\n", err)
			os.Exit(1)
		}
		if err != nil {
			os.Exit(1)
		}
		return
	}

	if deviceID == "" || secret == "" {
		fmt.Fprintln(os.Stderr, "device id and secret are required")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	queue, err := agent.NewSampleQueue(queuePath, int64(queueMaxMB)*1024*1024)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create queue: %v\n", err)
		os.Exit(1)
	}
	runner := agent.Runner{
		Client: &agent.Client{
			ServerURL: serverURL,
			DeviceID:  deviceID,
			Secret:    secret,
			Timeout:   10 * time.Second,
			UseGzip:   gzipBody,
		},
		Collector:        agent.NewCollector(nvidiaSMI, 5*time.Second),
		Queue:            queue,
		SampleInterval:   time.Duration(intervalSeconds) * time.Second,
		CollectProcesses: collectProcesses,
		Once:             once,
	}
	if err := runner.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "gpufleet-agent: %v\n", err)
		os.Exit(1)
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
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err == nil {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF":
		return false
	default:
		return fallback
	}
}
