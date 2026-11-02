package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

func Hide(cmd *exec.Cmd)          {}
func PanelPath(dir string) string { return filepath.Join(dir, "openflux-server-web") }
func IsAdmin() bool               { return os.Geteuid() == 0 }

func isEntware() bool {
	if _, err := exec.LookPath("systemctl"); err == nil {
		return false
	}
	if fi, err := os.Stat("/opt/etc/init.d/S99openflux"); err == nil && !fi.IsDir() {
		return true
	}
	if fi, err := os.Stat("/opt/etc/init.d"); err == nil && fi.IsDir() {
		return true
	}
	return false
}

func (c *Controller) Status(dir string) (Status, error) {
	if isEntware() {
		var s Status
		initScript := "/opt/etc/init.d/S99openflux"
		fi, err := os.Stat(initScript)
		if err != nil {
			return s, nil
		}
		s.Registered = true
		s.Enabled = (fi.Mode() & 0111) != 0
		s.Executable = PanelPath(dir)
		pidBytes, err := os.ReadFile("/opt/var/run/openflux.pid")
		if err == nil {
			pidStr := strings.TrimSpace(string(pidBytes))
			if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
				if proc, err := os.FindProcess(pid); err == nil {
					if proc.Signal(syscall.Signal(0)) == nil {
						if link, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid)); err == nil {
							if strings.Contains(link, "openflux-server-web") {
								s.Running = true
							}
						} else {
							s.Running = true
						}
					}
				}
			}
		}
		return s, nil
	}

	var s Status
	output, e := c.Run("systemctl", "show", ServiceName, "--no-pager", "--property=LoadState,ActiveState,UnitFileState,ExecStart")
	if e != nil {
		return s, e
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(output), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			values[k] = strings.TrimSpace(v)
		}
	}
	s.Registered = values["LoadState"] != "not-found" && values["LoadState"] != ""
	s.Running = values["ActiveState"] == "active" || values["ActiveState"] == "activating"
	s.Enabled = values["UnitFileState"] == "enabled" || values["UnitFileState"] == "enabled-runtime"
	if m := regexp.MustCompile(`path=([^;]+)`).FindStringSubmatch(values["ExecStart"]); len(m) > 1 {
		s.Executable = strings.TrimSpace(m[1])
	}
	return s, nil
}

func (c *Controller) Start(dir string) error {
	if isEntware() {
		initScript := "/opt/etc/init.d/S99openflux"
		if b, err := exec.Command(initScript, "start").CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s: %w", initScript, b, err)
		}
		return c.wait(dir, true)
	}
	if _, e := c.owned(dir); e != nil {
		return e
	}
	if _, e := c.Run("systemctl", "start", ServiceName); e != nil {
		return e
	}
	return c.wait(dir, true)
}

func (c *Controller) Stop(dir string) error {
	if isEntware() {
		initScript := "/opt/etc/init.d/S99openflux"
		if b, err := exec.Command(initScript, "stop").CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s: %w", initScript, b, err)
		}
		return c.wait(dir, false)
	}
	s, e := c.owned(dir)
	if e != nil {
		return e
	}
	if !s.Running {
		return nil
	}
	if _, e = c.Run("systemctl", "stop", ServiceName); e != nil {
		return e
	}
	return c.wait(dir, false)
}

func (c *Controller) Restart(dir string) error {
	if isEntware() {
		initScript := "/opt/etc/init.d/S99openflux"
		if b, err := exec.Command(initScript, "restart").CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s: %w", initScript, b, err)
		}
		return c.wait(dir, true)
	}
	if _, e := c.owned(dir); e != nil {
		return e
	}
	if _, e := c.Run("systemctl", "restart", ServiceName); e != nil {
		return e
	}
	return c.wait(dir, true)
}

func (c *Controller) Enable(dir string, on bool) error {
	if isEntware() {
		initScript := "/opt/etc/init.d/S99openflux"
		mode := os.FileMode(0755)
		if !on {
			mode = os.FileMode(0644)
		}
		return os.Chmod(initScript, mode)
	}
	if _, e := c.owned(dir); e != nil {
		return e
	}
	verb := "disable"
	if on {
		verb = "enable"
	}
	_, e := c.Run("systemctl", verb, ServiceName)
	return e
}

func (c *Controller) Remove(dir string) error {
	if isEntware() {
		_ = c.Stop(dir)
		_ = os.Remove("/opt/etc/init.d/S99openflux")
		_ = os.Remove("/opt/var/run/openflux.pid")
		return nil
	}
	if _, e := c.owned(dir); e != nil {
		return e
	}
	if e := c.Stop(dir); e != nil {
		return e
	}
	if _, e := c.Run("systemctl", "reset-failed", ServiceName); e != nil {
		return e
	}
	if _, e := c.Run("systemctl", "disable", ServiceName); e != nil {
		return e
	}
	unit := "/etc/systemd/system/" + ServiceName
	if e := os.Remove(unit); e != nil && !os.IsNotExist(e) {
		return e
	}
	_, e := c.Run("systemctl", "daemon-reload")
	return e
}
