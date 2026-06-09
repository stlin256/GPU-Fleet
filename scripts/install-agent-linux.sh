#!/usr/bin/env sh
set -eu

SERVER_URL="${SERVER_URL:-http://127.0.0.1:8080}"
DEVICE_ID="${DEVICE_ID:-}"
SECRET="${SECRET:-}"
INTERVAL="${INTERVAL:-10}"
QUEUE_MAX_MB="${QUEUE_MAX_MB:-128}"

if [ -z "$DEVICE_ID" ] || [ -z "$SECRET" ]; then
  echo "DEVICE_ID and SECRET are required. Create a device in the GPUFleet dashboard and pass its one-time secret explicitly." >&2
  exit 2
fi

if [ ! -f "./bin/gpufleet-agent" ]; then
  echo "Missing ./bin/gpufleet-agent. Build it first: GOOS=linux GOARCH=amd64 go build -o bin/gpufleet-agent ./cmd/gpufleet-agent" >&2
  exit 1
fi

install -d /usr/local/bin /etc/gpufleet /var/lib/gpufleet-agent
install -m 0755 ./bin/gpufleet-agent /usr/local/bin/gpufleet-agent
install -m 0644 ./scripts/gpufleet-agent.service /etc/systemd/system/gpufleet-agent.service

cat >/etc/gpufleet/agent.env <<EOF
GPUFLEET_SERVER_URL=${SERVER_URL}
GPUFLEET_DEVICE_ID=${DEVICE_ID}
GPUFLEET_SECRET=${SECRET}
GPUFLEET_INTERVAL=${INTERVAL}
GPUFLEET_QUEUE_MAX_MB=${QUEUE_MAX_MB}
EOF
chmod 0600 /etc/gpufleet/agent.env

systemctl daemon-reload
systemctl enable --now gpufleet-agent
systemctl status gpufleet-agent --no-pager
