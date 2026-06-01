#!/usr/bin/env sh
set -eu

systemctl disable --now gpufleet-agent 2>/dev/null || true
rm -f /etc/systemd/system/gpufleet-agent.service
systemctl daemon-reload

if [ "${REMOVE_FILES:-0}" = "1" ]; then
  rm -f /usr/local/bin/gpufleet-agent
  rm -rf /etc/gpufleet /var/lib/gpufleet-agent
fi

echo "Removed gpufleet-agent service"

