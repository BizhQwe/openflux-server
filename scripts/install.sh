#!/usr/bin/env bash
# ==============================================================================
# OpenFlux Server - Fast 1-Command Bootstrap Installer (Linux)
# Supports: Linux x64, Linux ARM64, Linux x86, Linux ARMv7
# ==============================================================================

set -eu
if (set -o pipefail) 2>/dev/null; then
    set -o pipefail
fi

if [ "$(id -u)" -ne 0 ]; then
    echo "  [ERROR] Root privileges required. Run with: sudo bash $0"
    exit 1
fi

for arg in "$@"; do
    if [ "$arg" = "--uninstall" ] || [ "$arg" = "uninstall" ] || [ "$arg" = "-u" ]; then
        echo ""
        echo "=============================================================="
        echo "  OpenFlux Server - Uninstall"
        echo "=============================================================="
        echo "  [INFO] Stopping services and processes..."
        systemctl disable --now openflux-server.service openflux-zrok.service openflux.service 2>/dev/null || true
        pkill -9 -x web 2>/dev/null || true
        pkill -9 -x OpenFlux.Server 2>/dev/null || true
        pkill -9 -x OpenFluxServer 2>/dev/null || true
        pkill -9 -x openflux-linux-amd64 2>/dev/null || true
        pkill -9 -x openflux-linux-arm64 2>/dev/null || true
        pkill -9 -x openflux-server-web 2>/dev/null || true
        pkill -9 -x openflux-server-core 2>/dev/null || true
        pkill -9 -x openflux-server-cli 2>/dev/null || true
        pkill -9 -x openflux 2>/dev/null || true
        pkill -9 -x zrok 2>/dev/null || true
        killall -9 openflux-server-web 2>/dev/null || true
        killall -9 openflux-server-core 2>/dev/null || true
        killall -9 openflux-server-cli 2>/dev/null || true
        killall -9 openflux 2>/dev/null || true
        killall -9 zrok 2>/dev/null || true

        echo "  [INFO] Removing systemd services..."
        rm -f /etc/systemd/system/openflux*.service /etc/systemd/system/multi-user.target.wants/openflux*.service /lib/systemd/system/openflux*.service
        systemctl daemon-reload 2>/dev/null || true
        systemctl reset-failed 2>/dev/null || true

        echo "  [INFO] Removing configurations and server files..."
        echo "  [INFO] Cleaning firewall rules..."
        ufw delete allow 5000/tcp 2>/dev/null || true
        if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
            firewall-cmd --remove-port=5000/tcp 2>/dev/null || true
            firewall-cmd --permanent --remove-port=5000/tcp 2>/dev/null || true
        fi
        rm -f /etc/nginx/conf.d/openflux*.conf /etc/nginx/sites-enabled/openflux* /etc/nginx/sites-available/openflux* 2>/dev/null || true
        systemctl reload nginx 2>/dev/null || true
        rm -f /etc/profile.d/openflux*.sh /etc/sudoers.d/openflux 2>/dev/null || true
        if [ -x /opt/etc/init.d/S99openflux ]; then /opt/etc/init.d/S99openflux stop 2>/dev/null || true; fi
        if [ -x /etc/init.d/openflux ]; then /etc/init.d/openflux stop 2>/dev/null || true; /etc/init.d/openflux disable 2>/dev/null || true; rm -f /etc/init.d/openflux; fi
        rm -f /opt/etc/init.d/S99openflux /opt/var/run/openflux.pid /opt/var/log/openflux.log /opt/bin/openflux
        rm -rf /opt/openflux-server /opt/openflux /etc/openflux /var/log/openflux /var/lib/openflux /tmp/openflux* /var/tmp/openflux* ~/.openflux ~/.config/openflux
        rm -f /usr/local/bin/OpenFluxServer /usr/local/bin/openfluxzenserver /usr/local/bin/openflux /usr/local/bin/openflux-server /usr/bin/OpenFluxServer /usr/bin/openfluxzenserver /usr/bin/openflux /usr/bin/openflux-server /bin/openflux 2>/dev/null || true

        echo ""
        echo "  [OK] OpenFlux Server was completely uninstalled."
        echo "=============================================================="
        echo ""
        exit 0
    fi
done

ARCH=$(uname -m)
case "$ARCH" in
    x86_64|amd64)
        RID="linux-amd64"
        DISPLAY_ARCH="linux-x64"
        ;;
    aarch64|arm64)
        RID="linux-arm64"
        DISPLAY_ARCH="linux-arm64"
        ;;
    i386|i486|i586|i686|x86)
        RID="linux-386"
        DISPLAY_ARCH="linux-x86"
        ;;
    armv7l|armv8l|armv7)
        RID="linux-armv7"
        DISPLAY_ARCH="linux-armv7"
        ;;
    *)
        echo "  [ERROR] Unsupported CPU architecture: $ARCH"
        exit 1
        ;;
esac

umask 077
TMP_BASE="/var/tmp"
[ -w "$TMP_BASE" ] || TMP_BASE="/tmp"
TMP_DIR=$(mktemp -d "$TMP_BASE/openflux-install.XXXXXXXXXX") || exit 1
ASSET="openflux-server-installer-$RID"
INSTALLER_BIN="$TMP_DIR/$ASSET"
CHECKSUM_FILE="$TMP_DIR/installer.sha256"
trap 'rm -f -- "$INSTALLER_BIN" "$CHECKSUM_FILE"; rmdir -- "$TMP_DIR"' EXIT
trap 'exit 1' HUP INT TERM
REPO="${OPENFLUX_REPOSITORY:-BizhQwe/openflux-server}"
TAG="${OPENFLUX_TAG:-v1.3.2-preview.2}"
RELEASE_BASE="${OPENFLUX_RELEASE_BASE:-https://github.com/$REPO/releases/download/$TAG}"
TAG_BASE="https://github.com/$REPO/releases/download/v1.3.2-preview.2"

if ! command -v curl >/dev/null 2>&1 && ! command -v wget >/dev/null 2>&1 && ! command -v uclient-fetch >/dev/null 2>&1; then
    echo '  [ERROR] curl or wget is required.' >&2
    echo '          Install curl via your package manager:' >&2
    echo '          OpenWrt / Entware:  opkg update && opkg install curl ca-certificates' >&2
    echo '          Debian / Ubuntu:    apt update && apt install -y curl' >&2
    echo '          Alpine Linux:       apk add curl' >&2
    exit 1
fi
if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1 && ! command -v openssl >/dev/null 2>&1; then
    echo '  [ERROR] sha256sum is required.' >&2
    exit 1
fi

download() {
    _url="$1"
    _out="$2"
    _maxsize="$3"
    if command -v curl >/dev/null 2>&1; then
        curl --proto '=https' --proto-redir '=https' --tlsv1.2 \
            --connect-timeout 15 --max-time 300 --max-filesize "$_maxsize" \
            --retry 2 -fsSL "$_url" -o "$_out"
    elif command -v wget >/dev/null 2>&1; then
        wget -q -T 300 -O "$_out" "$_url"
    elif command -v uclient-fetch >/dev/null 2>&1; then
        uclient-fetch -q -T 300 -O "$_out" "$_url"
    else
        return 1
    fi
}

verify_checksum() {
    _hash="$1"
    _file="$2"
    _dir="$3"
    if command -v sha256sum >/dev/null 2>&1; then
        (cd "$_dir" && printf '%s  %s\n' "$_hash" "$_file" | sha256sum --check --status 2>/dev/null || \
         cd "$_dir" && printf '%s  %s\n' "$_hash" "$_file" | sha256sum -c >/dev/null 2>&1)
    elif command -v shasum >/dev/null 2>&1; then
        (cd "$_dir" && printf '%s  %s\n' "$_hash" "$_file" | shasum -a 256 -c >/dev/null 2>&1)
    elif command -v openssl >/dev/null 2>&1; then
        _actual=$(openssl dgst -sha256 "$_dir/$_file" 2>/dev/null | awk '{print $NF}')
        [ "$_actual" = "$_hash" ]
    else
        return 1
    fi
}

echo ""
echo "  OpenFlux Server - Linux ($DISPLAY_ARCH)"
echo "  Downloading and verifying installer..."

DOWNLOADED=0
for base in "$RELEASE_BASE" "$TAG_BASE"; do
    if ! download "$base/$ASSET.sha256" "$CHECKSUM_FILE" 4096; then
        continue
    fi
    expected_hash=''
    expected_file=''
    extra=''
    read -r expected_hash expected_file extra < "$CHECKSUM_FILE" || [ -n "$expected_hash" ]
    expected_file=$(printf '%s' "$expected_file" | tr -d '\r')
    case "$expected_hash" in *[!0-9a-fA-F]*|'') echo '  [ERROR] Invalid SHA-256.' >&2; exit 1 ;; esac
    [ "${#expected_hash}" -eq 64 ] && [ -z "$extra" ] || { echo '  [ERROR] Invalid SHA-256.' >&2; exit 1; }
    case "$expected_file" in "$ASSET"|"*$ASSET") ;; *) echo '  [ERROR] Invalid checksum filename.' >&2; exit 1 ;; esac
    if ! download "$base/$ASSET" "$INSTALLER_BIN" 268435456; then
        continue
    fi
    [ -s "$INSTALLER_BIN" ] || { echo '  [ERROR] Empty installer.' >&2; exit 1; }
    if ! verify_checksum "$expected_hash" "$ASSET" "$TMP_DIR"; then
        echo '  [ERROR] Installer SHA-256 mismatch. Nothing was executed.' >&2
        exit 1
    fi
    DOWNLOADED=1
    break
done

if [ "$DOWNLOADED" -ne 1 ]; then
    echo "  [ERROR] Failed to download a verified installer for $DISPLAY_ARCH." >&2
    exit 1
fi
chmod 700 "$INSTALLER_BIN"
if [ -t 0 ]; then
    "$INSTALLER_BIN" "$@"
elif [ -c /dev/tty ] && ( : < /dev/tty ) 2>/dev/null; then
    "$INSTALLER_BIN" "$@" < /dev/tty
else
    "$INSTALLER_BIN" "$@"
fi
