#!/bin/sh
# ==============================================================================
# OpenFlux Server — установка на роутер с Entware (Keenetic / OpenWrt + /opt)
#
# Использование (на роутере от root):
#   sh install-entware.sh
#   sh install-entware.sh --port 8080
#   sh install-entware.sh --silent
#   sh install-entware.sh --uninstall
# ==============================================================================

set -eu

ARCH=$(uname -m)
case "$ARCH" in
    aarch64|arm64)        RID="linux-arm64"; DISPLAY_ARCH="linux-arm64" ;;
    armv7l|armv8l|armv7)  RID="linux-armv7"; DISPLAY_ARCH="linux-armv7" ;;
    x86_64|amd64)         RID="linux-amd64"; DISPLAY_ARCH="linux-x64" ;;
    i386|i486|i586|i686)  RID="linux-386";   DISPLAY_ARCH="linux-x86" ;;
    *)
        echo "  [ERROR] Неподдерживаемая архитектура процессора: $ARCH" >&2
        exit 1
        ;;
esac

[ "$(id -u)" -eq 0 ] || { echo "  [ERROR] Нужны права root (запустите через ssh root@...)" >&2; exit 1; }
for arg in "$@"; do
    if [ "$arg" = "--uninstall" ] || [ "$arg" = "uninstall" ] || [ "$arg" = "-u" ]; then
        echo ""
        echo "=============================================================="
        echo "  OpenFlux Server - Uninstall (Entware)"
        echo "=============================================================="
        echo "  [INFO] Stopping service and processes..."
        if [ -x /opt/etc/init.d/S99openflux ]; then
            /opt/etc/init.d/S99openflux stop 2>/dev/null || true
        fi
        pkill -9 -x openflux-server-web 2>/dev/null || true
        pkill -9 -x openflux-server-core 2>/dev/null || true
        pkill -9 -x openflux-server-cli 2>/dev/null || true
        pkill -9 -x openflux 2>/dev/null || true
        killall -9 openflux-server-web 2>/dev/null || true
        killall -9 openflux-server-core 2>/dev/null || true
        killall -9 openflux-server-cli 2>/dev/null || true
        killall -9 openflux 2>/dev/null || true

        echo "  [INFO] Removing Entware service..."
        rm -f /opt/etc/init.d/S99openflux /opt/var/run/openflux.pid /opt/var/log/openflux.log

        echo "  [INFO] Removing symlinks..."
        rm -f /opt/bin/openflux /usr/local/bin/openflux /usr/bin/openflux 2>/dev/null || true

        echo "  [INFO] Removing configuration and server files..."
        rm -rf /opt/openflux-server /opt/openflux /etc/openflux /var/log/openflux /tmp/openflux* /opt/tmp/openflux*

        echo ""
        echo "  [OK] OpenFlux Server was completely uninstalled."
        echo "=============================================================="
        echo ""
        exit 0
    fi
done
[ -d /opt/etc/init.d ] || { echo "  [ERROR] Entware не найден (/opt/etc/init.d отсутствует)." >&2; exit 1; }

if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1 && ! command -v uclient-fetch >/dev/null 2>&1; then
    echo "  [ERROR] Для загрузки требуется curl или wget." >&2
    echo "          Установите curl: opkg update && opkg install curl ca-certificates" >&2
    exit 1
fi
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1 && ! command -v openssl >/dev/null 2>&1; then
    echo "  [ERROR] Нужен sha256sum: opkg install coreutils-sha256sum" >&2
    exit 1
fi

calc_sha256() {
    _target="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$_target" | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$_target" | awk '{print $1}'
    elif command -v openssl >/dev/null 2>&1; then
        openssl dgst -sha256 "$_target" 2>/dev/null | awk '{print $NF}'
    fi
}

download_file() {
    _url="$1"
    _out="$2"
    _timeout="${3:-300}"
    if command -v curl >/dev/null 2>&1; then
        _proto="--proto =https --proto-redir =https --tlsv1.2"
        case "$_url" in
            http://*) _proto="--proto =https,http --proto-redir =https,http" ;;
        esac
        curl $_proto --connect-timeout 15 --max-time "$_timeout" --retry 2 -fsSL "$_url" -o "$_out"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -T "$_timeout" -O "$_out" "$_url"
    elif command -v uclient-fetch >/dev/null 2>&1; then
        uclient-fetch -q -T "$_timeout" -O "$_out" "$_url"
    else
        return 1
    fi
}

# Проверка памяти роутера
MEM_TOTAL=$(awk '/MemTotal/{print int($2/1024)}' /proc/meminfo 2>/dev/null || echo 0)
SWAP_TOTAL=$(awk 'NR>1{s+=$3} END{print int(s/1024)}' /proc/swaps 2>/dev/null || echo 0)
if [ "$MEM_TOTAL" -gt 0 ] && [ "$MEM_TOTAL" -lt 300 ] && [ "$SWAP_TOTAL" -lt 128 ]; then
    echo "  [WARN] Всего ${MEM_TOTAL} МБ RAM и swap < 128 МБ. Рекомендуется включить swap-файл на USB/диске."
fi

TMP_BASE="/tmp"
TMP_DIR=$(mktemp -d "$TMP_BASE/openflux-install.XXXXXX")
INSTALLER_BIN="$TMP_DIR/openflux-installer"
CHECKSUM_FILE="$TMP_DIR/installer.sha256"
trap 'rm -rf "$TMP_DIR"' EXIT HUP INT TERM

REPO="${OPENFLUX_REPOSITORY:-BizhQwe/openflux-server}"
ASSET="openflux-server-installer-$RID"
TAG="${OPENFLUX_TAG:-v1.3.2-preview.2}"
RELEASE_BASE="${OPENFLUX_RELEASE_BASE:-https://github.com/$REPO/releases/download/$TAG}"

echo ""
echo "  OpenFlux Server - Entware ($DISPLAY_ARCH)"
echo "  Downloading and verifying installer..."

download_file "$RELEASE_BASE/$ASSET.sha256" "$CHECKSUM_FILE" 300 || {
    echo "  [ERROR] Не удалось загрузить контрольную сумму релиза." >&2
    exit 1
}

EXPECTED=$(awk '{print $1; exit}' "$CHECKSUM_FILE" | tr -d '\r')
[ "${#EXPECTED}" -eq 64 ] || { echo "  [ERROR] Некорректная контрольная сумма SHA-256." >&2; exit 1; }

download_file "$RELEASE_BASE/$ASSET" "$INSTALLER_BIN" 600 || {
    echo "  [ERROR] Не удалось загрузить файл установщика." >&2
    exit 1
}

ACTUAL=$(calc_sha256 "$INSTALLER_BIN")
[ "$ACTUAL" = "$EXPECTED" ] || { echo "  [ERROR] Контрольная сумма SHA-256 не совпала. Установка прервана." >&2; exit 1; }

chmod 700 "$INSTALLER_BIN"

# Запуск единого Go-инсталлера в режиме --entware
if [ -t 0 ]; then
    "$INSTALLER_BIN" --entware "$@"
elif [ -c /dev/tty ] && ( : < /dev/tty ) 2>/dev/null; then
    "$INSTALLER_BIN" --entware "$@" < /dev/tty
else
    "$INSTALLER_BIN" --entware "$@"
fi
