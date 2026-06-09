#!/usr/bin/env sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root." >&2
  exit 1
fi

DATA_DIR="${DATA_DIR:-/var/lib/gpufleet}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/gpufleet}"
SERVICE_NAME="${SERVICE_NAME:-gpufleet-server}"
STOP_SERVICE="${STOP_SERVICE:-0}"

if [ ! -d "${DATA_DIR}" ]; then
  echo "Data directory does not exist: ${DATA_DIR}" >&2
  exit 1
fi

timestamp="$(date -u '+%Y%m%d-%H%M%S')"
archive="${BACKUP_DIR}/gpufleet-data-${timestamp}.tar.gz"
manifest="${BACKUP_DIR}/gpufleet-data-${timestamp}.manifest.txt"
data_parent="$(dirname "${DATA_DIR}")"
data_base="$(basename "${DATA_DIR}")"

mkdir -p "${BACKUP_DIR}"
umask 077

stopped=0
cleanup() {
  if [ "${stopped}" -eq 1 ] && command -v systemctl >/dev/null 2>&1; then
    systemctl start "${SERVICE_NAME}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

if [ "${STOP_SERVICE}" = "1" ] && command -v systemctl >/dev/null 2>&1; then
  systemctl stop "${SERVICE_NAME}"
  stopped=1
fi

{
  echo "product=GPUFleet"
  echo "created_at=$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo "host=$(hostname 2>/dev/null || printf unknown)"
  echo "data_dir=${DATA_DIR}"
  echo "service_name=${SERVICE_NAME}"
  echo "stop_service=${STOP_SERVICE}"
  if command -v du >/dev/null 2>&1; then
    echo "data_size_bytes=$(du -sb "${DATA_DIR}" 2>/dev/null | awk '{print $1}')"
  fi
} >"${manifest}"

tar -C "${data_parent}" \
  --exclude="${data_base}/*.tmp" \
  --exclude="${data_base}/*.lock" \
  -czf "${archive}" \
  "${data_base}"

if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "${archive}" >"${archive}.sha256"
fi

echo "Backup created: ${archive}"
echo "Manifest: ${manifest}"
if [ -f "${archive}.sha256" ]; then
  echo "SHA256: ${archive}.sha256"
fi
