#!/usr/bin/env sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root." >&2
  exit 1
fi

REPO_DIR="${REPO_DIR:-$(pwd -P)}"
INSTALL_DIR="${INSTALL_DIR:-/opt/gpufleet}"
BIN_PATH="${BIN_PATH:-${INSTALL_DIR}/gpufleet-server}"
PREBUILT_BIN="${PREBUILT_BIN:-${REPO_DIR}/bin/gpufleet-server}"
DATA_DIR="${DATA_DIR:-/var/lib/gpufleet}"
WEB_DIR="${WEB_DIR:-${REPO_DIR}/web/dist}"
ADDR="${ADDR:-0.0.0.0:9008}"
MIN_FREE_MB="${MIN_FREE_MB:-800}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
SERVICE_NAME="${SERVICE_NAME:-gpufleet-server}"
ENV_DIR="${ENV_DIR:-/etc/gpufleet}"
ENV_FILE="${ENV_FILE:-${ENV_DIR}/server.env}"
UNIT_FILE="/etc/systemd/system/${SERVICE_NAME}.service"

USE_PREBUILT=0
if [ -f "${PREBUILT_BIN}" ]; then
  USE_PREBUILT=1
fi

if [ "${USE_PREBUILT}" -eq 0 ]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "Missing go and no prebuilt server binary found at ${PREBUILT_BIN}." >&2
    echo "Install Go for source deployments, or run this script from a release package." >&2
    exit 1
  fi
  if [ ! -f "${REPO_DIR}/go.mod" ] || [ ! -d "${REPO_DIR}/cmd/gpufleet-server" ]; then
    echo "REPO_DIR must point to a GPUFleet Git checkout when no prebuilt binary is present. Current value: ${REPO_DIR}" >&2
    exit 1
  fi
fi

if [ ! -f "${WEB_DIR}/index.html" ]; then
  echo "Missing web build at ${WEB_DIR}." >&2
  echo "Use the committed web/dist directory or run: cd ${REPO_DIR}/web && npm install && npm run build" >&2
  exit 1
fi

mkdir -p "${INSTALL_DIR}" "${DATA_DIR}" "${ENV_DIR}"

COMMIT="dev"
if command -v git >/dev/null 2>&1; then
  COMMIT="$(git -C "${REPO_DIR}" rev-parse HEAD 2>/dev/null || printf dev)"
fi
BUILD_TIME="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"

if [ "${USE_PREBUILT}" -eq 1 ]; then
  echo "Installing prebuilt GPUFleet server from ${PREBUILT_BIN}..."
  cp "${PREBUILT_BIN}" "${BIN_PATH}"
else
  echo "Building GPUFleet server from ${REPO_DIR}..."
  (
    cd "${REPO_DIR}"
    go build \
      -ldflags "-X gpufleet/internal/version.Commit=${COMMIT} -X gpufleet/internal/version.BuildTime=${BUILD_TIME}" \
      -o "${BIN_PATH}" \
      ./cmd/gpufleet-server
  )
fi
chmod 0755 "${BIN_PATH}"

cat >"${ENV_FILE}" <<EOF
GPUFLEET_ADDR=${ADDR}
GPUFLEET_DATA_DIR=${DATA_DIR}
GPUFLEET_WEB_DIR=${WEB_DIR}
GPUFLEET_REPO_DIR=${REPO_DIR}
GPUFLEET_MIN_FREE_MB=${MIN_FREE_MB}
GPUFLEET_RETENTION_DAYS=${RETENTION_DAYS}
EOF

if [ "${ADMIN_PASSWORD:-}" != "" ]; then
  printf 'GPUFLEET_ADMIN_PASSWORD=%s\n' "${ADMIN_PASSWORD}" >>"${ENV_FILE}"
fi
if [ "${BOOTSTRAP_DEVICE_ID:-}" != "" ]; then
  printf 'GPUFLEET_BOOTSTRAP_DEVICE_ID=%s\n' "${BOOTSTRAP_DEVICE_ID}" >>"${ENV_FILE}"
fi
if [ "${BOOTSTRAP_SECRET:-}" != "" ]; then
  printf 'GPUFLEET_BOOTSTRAP_SECRET=%s\n' "${BOOTSTRAP_SECRET}" >>"${ENV_FILE}"
fi
chmod 0600 "${ENV_FILE}"

cat >"${UNIT_FILE}" <<EOF
[Unit]
Description=GPUFleet Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${ENV_FILE}
WorkingDirectory=${REPO_DIR}
ExecStart=${BIN_PATH} \\
  -addr \${GPUFLEET_ADDR} \\
  -data-dir \${GPUFLEET_DATA_DIR} \\
  -web-dir \${GPUFLEET_WEB_DIR} \\
  -repo-dir \${GPUFLEET_REPO_DIR} \\
  -min-free-mb \${GPUFLEET_MIN_FREE_MB} \\
  -retention-days \${GPUFLEET_RETENTION_DAYS}
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ReadWritePaths=${DATA_DIR} ${INSTALL_DIR} ${REPO_DIR}

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now "${SERVICE_NAME}"

echo "GPUFleet server installed."
echo "Service: ${SERVICE_NAME}"
echo "Address: ${ADDR}"
echo "Data directory: ${DATA_DIR}"
echo "Web directory: ${WEB_DIR}"
echo "Repository directory: ${REPO_DIR}"
systemctl status "${SERVICE_NAME}" --no-pager
