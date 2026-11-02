#!/bin/sh
# ==============================================================================
# OpenFlux Server — установка на роутер с Entware (Keenetic / OpenWrt + /opt)
#
# Отличия от scripts/install.sh (systemd-версия):
#   * нет systemd — служба ставится как /opt/etc/init.d/S99openflux
#   * всё живёт в /opt (USB-накопитель), а не в /usr/local
#   * порт панели открывается в iptables ПЕРЕД цепочкой _NDM_ACL_IN,
#     иначе Keenetic режет входящие (-P INPUT DROP)
#   * ограничение памяти через GOMEMLIMIT/GOGC (на роутере ~500 МБ RAM)
#   * проверка свопа, места на /opt и занятости порта до установки
#
# Использование (на роутере, от root):
#   sh install-entware.sh                 # LAN + localtunnel, порт 5000
#   sh install-entware.sh --lan-only      # только локальная сеть
#   sh install-entware.sh --port 8080
#   sh install-entware.sh --uninstall
#   curl -fsSL <raw-url> | sh
# ==============================================================================

set -eu

REPO="${OPENFLUX_REPOSITORY:-BizhQwe/openflux-server}"
INSTALL_DIR="${OPENFLUX_DIR:-/opt/openflux-server}"
PORT=5000
MODE="localtunnel"        # localtunnel | lan | localhost
LANG_CODE="ru"
ASSUME_YES=0
ACTION="install"

INIT_SCRIPT="/opt/etc/init.d/S99openflux"
LOG_DIR="/opt/var/log"
LOG_FILE="$LOG_DIR/openflux.log"
PID_FILE="/opt/var/run/openflux.pid"
CLI_LINK="/opt/bin/openflux"

# ------------------------------------------------------------------ аргументы
while [ $# -gt 0 ]; do
    case "$1" in
        --uninstall|-u|uninstall) ACTION="uninstall" ;;
        --port) shift; PORT="${1:-}" ;;
        --port=*) PORT="${1#*=}" ;;
        --lan-only) MODE="lan" ;;
        --localtunnel) MODE="localtunnel" ;;
        --localhost) MODE="localhost" ;;
        --mode) shift; MODE="${1:-}" ;;
        --mode=*) MODE="${1#*=}" ;;
        --dir) shift; INSTALL_DIR="${1:-}" ;;
        --dir=*) INSTALL_DIR="${1#*=}" ;;
        --lang) shift; LANG_CODE="${1:-}" ;;
        --lang=*) LANG_CODE="${1#*=}" ;;
        -y|--yes) ASSUME_YES=1 ;;
        -h|--help)
            sed -n '2,22p' "$0" 2>/dev/null || true
            exit 0 ;;
        *) echo "  [ERROR] Неизвестный аргумент: $1" >&2; exit 1 ;;
    esac
    shift
done

say()  { printf '  %s\n' "$*"; }
ok()   { printf '  [OK] %s\n' "$*"; }
warn() { printf '  [WARN] %s\n' "$*"; }
die()  { printf '  [ERROR] %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "Нужны права root. Запустите через ssh root@192.168.1.1 -p 222"

# --------------------------------------------------------------- окружение
[ -d /opt/etc/init.d ] || die "Entware не найден (/opt/etc/init.d отсутствует)"
command -v curl >/dev/null 2>&1      || die "Нужен curl: opkg install curl"
command -v sha256sum >/dev/null 2>&1 || die "Нужен sha256sum: opkg install coreutils-sha256sum"

# --------------------------------------------------------------- удаление
if [ "$ACTION" = "uninstall" ]; then
    say "Остановка службы..."
    [ -x "$INIT_SCRIPT" ] && "$INIT_SCRIPT" stop >/dev/null 2>&1 || true
    pkill -f "$INSTALL_DIR/openflux-server-web" 2>/dev/null || true
    pkill -f "$INSTALL_DIR/openflux-server-core" 2>/dev/null || true
    sleep 1

    say "Закрытие порта в iptables..."
    while iptables -D INPUT -p tcp --dport "$PORT" -j ACCEPT 2>/dev/null; do :; done

    say "Удаление файлов..."
    rm -f "$INIT_SCRIPT" "$PID_FILE" "$CLI_LINK"
    rm -rf "$INSTALL_DIR"

    ok "OpenFlux Server удалён. Логи остались в $LOG_FILE"
    exit 0
fi

case "$LANG_CODE" in ru|en) ;; *) die "--lang должен быть ru или en" ;; esac
case "$PORT" in ''|*[!0-9]*) die "--port должен быть числом" ;; esac
[ "$PORT" -ge 1 ] && [ "$PORT" -le 65535 ] || die "--port вне диапазона 1-65535"
case "$MODE" in localtunnel|lan|localhost) ;; *) die "--mode: localtunnel, lan или localhost" ;; esac

# ------------------------------------------------------------------ архитектура
ARCH=$(uname -m)
case "$ARCH" in
    aarch64|arm64)        RID="linux-arm64" ;;
    armv7l|armv8l|armv7)  RID="linux-armv7" ;;
    x86_64|amd64)         RID="linux-amd64" ;;
    i386|i486|i586|i686)  RID="linux-386" ;;
    *) die "Неподдерживаемая архитектура: $ARCH" ;;
esac

# ------------------------------------------------------------------ проверки
say "Проверка ресурсов..."
MEM_TOTAL=$(awk '/MemTotal/{print int($2/1024)}' /proc/meminfo)
MEM_AVAIL=$(awk '/MemAvailable/{print int($2/1024)}' /proc/meminfo)
SWAP_TOTAL=$(awk 'NR>1{s+=$3} END{print int(s/1024)}' /proc/swaps 2>/dev/null || echo 0)

say "RAM: ${MEM_TOTAL} МБ всего, ${MEM_AVAIL} МБ доступно; swap: ${SWAP_TOTAL} МБ"
if [ "$MEM_TOTAL" -lt 300 ]; then
    warn "Мало RAM. Панель может не запуститься — включите swap."
fi
if [ "$SWAP_TOTAL" -lt 256 ]; then
    warn "Swap < 256 МБ. Рекомендуется файл подкачки на /opt (см. README)."
fi

mkdir -p "$LOG_DIR" /opt/var/run
FREE_KB=$(df -k /opt | awk 'NR==2{print $4}')
if [ "$FREE_KB" -lt 60000 ]; then
    warn "На /opt свободно $((FREE_KB/1024)) МБ — впритык, бинарники ~40-60 МБ."
fi

# Порт занят?
if netstat -ltn 2>/dev/null | grep -q ":$PORT "; then
    die "Порт $PORT уже занят. Выберите другой: --port 8080"
fi

# ------------------------------------------------------------ установка файлов
say "Загрузка установщика OpenFlux ($RID)..."
TMP_DIR=$(mktemp -d /opt/tmp/openflux.XXXXXX 2>/dev/null || mktemp -d /tmp/openflux.XXXXXX)
trap 'rm -rf "$TMP_DIR"' EXIT HUP INT TERM

ASSET="openflux-server-installer-$RID"
BASE="https://github.com/$REPO/releases/latest/download"

fetch() {
    curl --proto '=https' --proto-redir '=https' --tlsv1.2 \
         --connect-timeout 15 --max-time 600 --retry 2 -fsSL "$1" -o "$2"
}

fetch "$BASE/$ASSET.sha256" "$TMP_DIR/sum" || die "Не удалось скачать контрольную сумму"
EXPECTED=$(awk '{print $1; exit}' "$TMP_DIR/sum" | tr -d '\r')
case "$EXPECTED" in
    *[!0-9a-fA-F]*|'') die "Некорректная SHA-256 в файле релиза" ;;
esac
[ "${#EXPECTED}" -eq 64 ] || die "Некорректная длина SHA-256"

fetch "$BASE/$ASSET" "$TMP_DIR/installer" || die "Не удалось скачать установщик"
ACTUAL=$(sha256sum "$TMP_DIR/installer" | awk '{print $1}')
[ "$ACTUAL" = "$EXPECTED" ] || die "SHA-256 не совпала — установка прервана"
chmod 700 "$TMP_DIR/installer"
ok "Установщик проверен"

say "Установка в $INSTALL_DIR (режим: $MODE, порт: $PORT)..."
# --no-service: systemd на роутере нет, службу создаём ниже сами.
"$TMP_DIR/installer" \
    --silent \
    --no-service \
    --dir "$INSTALL_DIR" \
    --mode "$MODE" \
    --port "$PORT" \
    --lang "$LANG_CODE" 2>&1 | tee "$LOG_DIR/openflux-install.log" || {
        die "Установщик завершился с ошибкой (см. $LOG_DIR/openflux-install.log)"
    }

[ -x "$INSTALL_DIR/openflux-server-web" ] || die "Панель не установлена"
ok "Файлы установлены"

# --------------------------------------------------------------- init-скрипт
say "Создание службы $INIT_SCRIPT..."
cat > "$INIT_SCRIPT" <<'INIT_EOF'
#!/bin/sh
# OpenFlux Server — служба для Entware (Keenetic / OpenWrt)
# Управление: /opt/etc/init.d/S99openflux {start|stop|restart|status|firewall}

DESC="OpenFlux Server"
NAME=openflux-server-web
DAEMON=__DAEMON__
DATA_DIR=__DATA_DIR__
PORT=__PORT__
PIDFILE=__PIDFILE__
LOGFILE=__LOGFILE__
ENABLED=yes

# Ограничения под слабый роутер: мягкий лимит памяти и частый GC.
export GOMEMLIMIT=192MiB
export GOGC=50

running() { [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE" 2>/dev/null)" 2>/dev/null; }

# Порт открываем перед _NDM_ACL_IN: у Keenetic политика INPUT = DROP,
# и правило, добавленное в конец, не сработает.
firewall_open() {
    while iptables -D INPUT -p tcp --dport "$PORT" -j ACCEPT 2>/dev/null; do :; done
    iptables -I INPUT 1 -p tcp --dport "$PORT" -j ACCEPT 2>/dev/null || \
        iptables -I INPUT -p tcp --dport "$PORT" -j ACCEPT 2>/dev/null || true
    ip6tables -I INPUT 1 -p tcp --dport "$PORT" -j ACCEPT 2>/dev/null || true
}
firewall_close() {
    while iptables -D INPUT -p tcp --dport "$PORT" -j ACCEPT 2>/dev/null; do :; done
    while ip6tables -D INPUT -p tcp --dport "$PORT" -j ACCEPT 2>/dev/null; do :; done
}

start() {
    [ "$ENABLED" = "yes" ] || { echo "$DESC disabled"; return 8; }
    if running; then echo "$DESC already running (pid $(cat "$PIDFILE"))"; return 0; fi
    firewall_open
    mkdir -p "$(dirname "$PIDFILE")" "$(dirname "$LOGFILE")"
    echo -n "Starting $DESC: "
    nohup "$DAEMON" --data-dir "$DATA_DIR" >>"$LOGFILE" 2>&1 &
    echo $! > "$PIDFILE"
    sleep 2
    if running; then echo "ok (pid $(cat "$PIDFILE"))"; return 0; fi
    rm -f "$PIDFILE"
    echo "failed (см. $LOGFILE)"
    tail -n 5 "$LOGFILE" 2>/dev/null
    return 255
}

stop() {
    firewall_close
    if ! running; then echo "$DESC not running"; rm -f "$PIDFILE"; return 0; fi
    echo -n "Stopping $DESC: "
    PID=$(cat "$PIDFILE")
    kill -TERM "$PID" 2>/dev/null
    i=0
    while kill -0 "$PID" 2>/dev/null && [ $i -lt 15 ]; do sleep 1; i=$((i+1)); done
    kill -9 "$PID" 2>/dev/null || true
    rm -f "$PIDFILE"
    echo "ok"
}

status() {
    if running; then
        echo "$DESC running (pid $(cat "$PIDFILE"))"
        echo "URL: http://$(nvram get lan_ipaddr 2>/dev/null || ip -4 addr show br0 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1):$PORT/"
    else
        echo "$DESC stopped"; return 1
    fi
}

case "$1" in
    start)     start ;;
    stop)      stop ;;
    restart)   stop; sleep 1; start ;;
    status)    status ;;
    firewall)  firewall_open; echo "port $PORT opened" ;;
    *) echo "Usage: $0 {start|stop|restart|status|firewall}"; exit 1 ;;
esac
INIT_EOF

# Подстановка путей в шаблон
sed -i \
    -e "s|__DAEMON__|$INSTALL_DIR/openflux-server-web|" \
    -e "s|__DATA_DIR__|$INSTALL_DIR/data|" \
    -e "s|__PORT__|$PORT|" \
    -e "s|__PIDFILE__|$PID_FILE|" \
    -e "s|__LOGFILE__|$LOG_FILE|" \
    "$INIT_SCRIPT" 2>/dev/null || {
        # На некоторых сборках busybox sed есть, но -i работает иначе
        TMP_INIT="$TMP_DIR/S99openflux"
        sed \
            -e "s|__DAEMON__|$INSTALL_DIR/openflux-server-web|" \
            -e "s|__DATA_DIR__|$INSTALL_DIR/data|" \
            -e "s|__PORT__|$PORT|" \
            -e "s|__PIDFILE__|$PID_FILE|" \
            -e "s|__LOGFILE__|$LOG_FILE|" \
            "$INIT_SCRIPT" > "$TMP_INIT" && mv "$TMP_INIT" "$INIT_SCRIPT"
    }
chmod 755 "$INIT_SCRIPT"

# CLI в PATH, чтобы работали `openflux network status` и т.п.
if [ -x "$INSTALL_DIR/openflux-server-cli" ]; then
    ln -sf "$INSTALL_DIR/openflux-server-cli" "$CLI_LINK"
    ok "Команда: $CLI_LINK"
fi

# --------------------------------------------------------------- запуск
say "Запуск службы..."
"$INIT_SCRIPT" start || die "Служба не запустилась. Лог: $LOG_FILE"

say "Ожидание готовности панели на порту $PORT..."
i=0
while [ $i -lt 30 ]; do
    if curl -fsS --max-time 2 -o /dev/null "http://127.0.0.1:$PORT/" 2>/dev/null; then break; fi
    sleep 1; i=$((i+1))
done

if curl -fsS --max-time 2 -o /dev/null "http://127.0.0.1:$PORT/" 2>/dev/null; then
    ok "Панель отвечает"
else
    warn "Панель пока не отвечает. Проверьте: $INIT_SCRIPT status; tail -20 $LOG_FILE"
fi

# --------------------------------------------------------------- итог
LAN_IP=$(ip -4 addr show br0 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1)
[ -n "$LAN_IP" ] || LAN_IP=$(ip -4 route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i=="src") print $(i+1)}')

echo ""
echo "=============================================================="
echo "  OpenFlux Server установлен"
echo "=============================================================="
[ -n "$LAN_IP" ] && echo "  Из локальной сети (телефон в Wi-Fi): http://$LAN_IP:$PORT/"
echo "  Проверка: $INIT_SCRIPT status"
echo "  Логи:     $LOG_FILE"
echo "  Логин/пароль: см. $LOG_DIR/openflux-install.log"
echo ""
echo "  Служба стартует автоматически: $INIT_SCRIPT (S99 = после сети)"
echo "=============================================================="
echo ""
