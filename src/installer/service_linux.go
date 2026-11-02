package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var forceEntware bool

func isEntware() bool {
	if forceEntware {
		return true
	}
	if _, err := exec.LookPath("systemctl"); err == nil {
		return false
	}
	fi, err := os.Stat("/opt/etc/init.d")
	return err == nil && fi.IsDir()
}

func defaultInstallDir() string { return "/opt/openflux-server" }
func hideProcess(cmd *exec.Cmd) {}
func protectData(dir string) error {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	return os.Chmod(dir, 0700)
}
func preflightService() error {
	if os.Geteuid() != 0 {
		return errors.New("run this installer with sudo, or use --no-service --dir <directory>")
	}
	if isEntware() {
		return nil
	}
	if _, e := exec.LookPath("systemctl"); e != nil {
		return errors.New("systemd is unavailable; use --no-service and run the panel with your process supervisor")
	}
	return nil
}
func stopService() error {
	if isEntware() {
		initScript := "/opt/etc/init.d/S99openflux"
		if _, err := os.Stat(initScript); err == nil {
			_ = exec.Command(initScript, "stop").Run()
		}
		return nil
	}
	for _, svc := range []string{"openflux-server.service", "openflux-zen-server.service"} {
		cmd := exec.Command("systemctl", "is-active", "--quiet", svc)
		if cmd.Run() == nil {
			if b, e := exec.Command("systemctl", "stop", svc).CombinedOutput(); e != nil {
				return fmt.Errorf("stop service %s: %s: %w", svc, b, e)
			}
		}
	}
	return nil
}
func registerService(dir string, autostart bool) error {
	if isEntware() {
		return registerEntwareService(dir, autostart)
	}
	unit, e := systemdUnit(dir)
	if e != nil {
		return e
	}
	if _, err := os.Stat("/etc/systemd/system/openflux-zen-server.service"); err == nil {
		_ = exec.Command("systemctl", "disable", "openflux-zen-server.service").Run()
		_ = os.Remove("/etc/systemd/system/openflux-zen-server.service")
	}
	if e := os.WriteFile("/etc/systemd/system/openflux-server.service", []byte(unit), 0644); e != nil {
		return e
	}
	boot := "disable"
	if autostart {
		boot = "enable"
	}
	for _, args := range [][]string{{"daemon-reload"}, {boot, "openflux-server.service"}} {
		if b, e := exec.Command("systemctl", args...).CombinedOutput(); e != nil {
			return fmt.Errorf("systemctl: %s: %w", b, e)
		}
	}
	return nil
}

func registerEntwareService(dir string, autostart bool) error {
	if err := os.MkdirAll("/opt/etc/init.d", 0755); err != nil {
		return err
	}
	if err := os.MkdirAll("/opt/var/run", 0755); err != nil {
		return err
	}
	if err := os.MkdirAll("/opt/var/log", 0755); err != nil {
		return err
	}
	daemon := filepath.Join(dir, "openflux-server-web")
	dataDir := filepath.Join(dir, "data")
	enabled := "yes"
	if !autostart {
		enabled = "no"
	}
	content := fmt.Sprintf(`#!/bin/sh
# OpenFlux Server — init script for Entware (Keenetic / OpenWrt)
# Usage: /opt/etc/init.d/S99openflux {start|stop|restart|status}

DESC="OpenFlux Server"
NAME=openflux-server-web
DAEMON="%s"
DATA_DIR="%s"
PIDFILE="/opt/var/run/openflux.pid"
LOGFILE="/opt/var/log/openflux.log"
ENABLED=%s

# Memory optimization for routers (soft memory limit & frequent GC)
export GOMEMLIMIT=192MiB
export GOGC=50

running() { [ -f "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE" 2>/dev/null)" 2>/dev/null; }

start() {
    [ "$ENABLED" = "yes" ] || { echo "$DESC disabled"; return 0; }
    if running; then echo "$DESC already running (pid $(cat "$PIDFILE"))"; return 0; fi
    mkdir -p "$(dirname "$PIDFILE")" "$(dirname "$LOGFILE")" "$DATA_DIR/logs"
    echo -n "Starting $DESC: "
    nohup "$DAEMON" --data-dir "$DATA_DIR" >>"$LOGFILE" 2>&1 &
    echo $! > "$PIDFILE"
    sleep 2
    if running; then echo "ok (pid $(cat "$PIDFILE"))"; return 0; fi
    rm -f "$PIDFILE"
    echo "failed (see $LOGFILE)"
    return 1
}

stop() {
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
    else
        echo "$DESC stopped"; return 1
    fi
}

case "$1" in
    start)   start ;;
    stop)    stop ;;
    restart) stop; sleep 1; start ;;
    status)  status ;;
    *) echo "Usage: $0 {start|stop|restart|status}"; exit 1 ;;
esac
`, daemon, dataDir, enabled)

	initScript := "/opt/etc/init.d/S99openflux"
	mode := os.FileMode(0755)
	if !autostart {
		mode = os.FileMode(0644)
	}
	return os.WriteFile(initScript, []byte(content), mode)
}
