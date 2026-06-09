#!/usr/bin/env sh
set -eu

if [ "$(id -u)" -ne 0 ]; then
  echo "Run as root." >&2
  exit 1
fi

BACKUP_FILE="${1:-${BACKUP_FILE:-}}"
DATA_DIR="${DATA_DIR:-/var/lib/gpufleet}"
SERVICE_NAME="${SERVICE_NAME:-gpufleet-server}"
START_SERVICE="${START_SERVICE:-1}"
CONFIRM_RESTORE="${CONFIRM_RESTORE:-0}"

if [ "${CONFIRM_RESTORE}" != "1" ]; then
  echo "Set CONFIRM_RESTORE=1 to restore server data." >&2
  exit 1
fi

if [ "${BACKUP_FILE}" = "" ] || [ ! -f "${BACKUP_FILE}" ]; then
  echo "Usage: CONFIRM_RESTORE=1 BACKUP_FILE=/path/gpufleet-data-YYYYmmdd-HHMMSS.tar.gz sh ./scripts/restore-server-linux.sh" >&2
  echo "   or: CONFIRM_RESTORE=1 sh ./scripts/restore-server-linux.sh /path/gpufleet-data-YYYYmmdd-HHMMSS.tar.gz" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1 && [ -f "${BACKUP_FILE}.sha256" ]; then
  sha256sum -c "${BACKUP_FILE}.sha256"
fi

timestamp="$(date -u '+%Y%m%d-%H%M%S')"
data_parent="$(dirname "${DATA_DIR}")"
rollback_dir="${DATA_DIR}.pre-restore-${timestamp}"
tmp_dir="$(mktemp -d)"

cleanup() {
  rm -rf "${tmp_dir}"
}
trap cleanup EXIT

if command -v systemctl >/dev/null 2>&1; then
  systemctl stop "${SERVICE_NAME}" || true
fi

tar -xzf "${BACKUP_FILE}" -C "${tmp_dir}"
restored_dir="$(find "${tmp_dir}" -mindepth 1 -maxdepth 1 -type d | sed -n '1p')"
if [ "${restored_dir}" = "" ]; then
  echo "Backup archive does not contain a data directory." >&2
  exit 1
fi

mkdir -p "${data_parent}"
if [ -e "${DATA_DIR}" ]; then
  mv "${DATA_DIR}" "${rollback_dir}"
  echo "Existing data moved to: ${rollback_dir}"
fi

mv "${restored_dir}" "${DATA_DIR}"
chmod -R go-rwx "${DATA_DIR}" 2>/dev/null || true

if [ "${START_SERVICE}" = "1" ] && command -v systemctl >/dev/null 2>&1; then
  systemctl start "${SERVICE_NAME}"
fi

echo "Restore completed: ${DATA_DIR}"
if [ -d "${rollback_dir}" ]; then
  echo "Rollback copy: ${rollback_dir}"
fi
