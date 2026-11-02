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
[ -d /opt/etc/init.d ] || { echo "  [ERROR] Entware не найден (/opt/etc/init.d отсутствует)." >&2; exit 1; }

command -v curl >/dev/null 2>&1      || { echo "  [ERROR] Нужен curl: opkg install curl" >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1 || { echo "  [ERROR] Нужен sha256sum: opkg install coreutils-sha256sum" >&2; exit 1; }

# Проверка памяти роутера
MEM_TOTAL=$(awk '/MemTotal/{print int($2/1024)}' /proc/meminfo 2>/dev/null || echo 0)
SWAP_TOTAL=$(awk 'NR>1{s+=$3} END{print int(s/1024)}' /proc/swaps 2>/dev/null || echo 0)
if [ "$MEM_TOTAL" -gt 0 ] && [ "$MEM_TOTAL" -lt 300 ] && [ "$SWAP_TOTAL" -lt 128 ]; then
    echo "  [WARN] Всего ${MEM_TOTAL} МБ RAM и swap < 128 МБ. Рекомендуется включить swap-файл на USB/диске."
fi

TMP_BASE="/opt/tmp"
[ -d "$TMP_BASE" ] && [ -w "$TMP_BASE" ] || TMP_BASE="/tmp"
TMP_DIR=$(mktemp -d "$TMP_BASE/openflux-install.XXXXXX")
INSTALLER_BIN="$TMP_DIR/openflux-installer"
CHECKSUM_FILE="$TMP_DIR/installer.sha256"
trap 'rm -rf "$TMP_DIR"' EXIT HUP INT TERM

REPO="${OPENFLUX_REPOSITORY:-BizhQwe/openflux-server}"
ASSET="openflux-server-installer-$RID"
RELEASE_BASE="https://github.com/$REPO/releases/latest/download"

echo ""
echo "  OpenFlux Server — Entware ($DISPLAY_ARCH)"
echo "  Загрузка и проверка установщика..."

curl --proto '=https' --proto-redir '=https' --tlsv1.2 \
    --connect-timeout 15 --max-time 300 --retry 2 -fsSL \
    "$RELEASE_BASE/$ASSET.sha256" -o "$CHECKSUM_FILE" || {
        echo "  [ERROR] Не удалось загрузить контрольную сумму релиза." >&2
        exit 1
    }

EXPECTED=$(awk '{print $1; exit}' "$CHECKSUM_FILE" | tr -d '\r')
[ "${#EXPECTED}" -eq 64 ] || { echo "  [ERROR] Некорректная контрольная сумма SHA-256." >&2; exit 1; }

curl --proto '=https' --proto-redir '=https' --tlsv1.2 \
    --connect-timeout 15 --max-time 600 --retry 2 -fsSL \
    "$RELEASE_BASE/$ASSET" -o "$INSTALLER_BIN" || {
        echo "  [ERROR] Не удалось загрузить файл установщика." >&2
        exit 1
    }

ACTUAL=$(sha256sum "$INSTALLER_BIN" | awk '{print $1}')
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
