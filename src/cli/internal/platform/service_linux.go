package platform

import (

	"fmt"

	"os"

	"os/exec"

	"path/filepath"

	"regexp"

	"strconv"

	"strings"

)

func Hide(cmd *exec.Cmd)          {}

func PanelPath(dir string) string { return filepath.Join(dir, "openflux-server-web") }

func IsAdmin() bool               { return os.Geteuid() == 0 }

func isEntware() bool {

	if fi, err := os.Stat("/opt/etc/init.d/S99openflux"); err == nil && !fi.IsDir() {

		return true

	}

	if _, err := exec.LookPath("systemctl"); err == nil {

		return false

	}

	if fi, err := os.Stat("/opt/etc/init.d"); err == nil && fi.IsDir() {

		return true

	}

	return false

}

func isOpenWrt() bool {

	if isEntware() {

		return false

	}

	if _, err := os.Stat("/etc/init.d/openflux"); err == nil {

		return true

	}

	if _, err := os.Stat("/etc/openwrt_release"); err == nil {

		return true

	}

	if _, err := os.Stat("/etc/rc.common"); err == nil {

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

				if fi, statErr := os.Stat(fmt.Sprintf("/proc/%d", pid)); statErr == nil && fi.IsDir() {

					s.Running = true

				}

			}

		}

		return s, nil

	}

	if isOpenWrt() {

		var s Status

		initScript := "/etc/init.d/openflux"

		if _, err := os.Stat(initScript); err != nil {

			return s, nil

		}

		s.Registered = true

		s.Executable = PanelPath(dir)

		cmd := exec.Command(initScript, "enabled")

		s.Enabled = cmd.Run() == nil

		cmd = exec.Command(initScript, "running")

		if cmd.Run() == nil {

			s.Running = true

		} else {

			out, err := exec.Command("pidof", "openflux-server-web").CombinedOutput()

			s.Running = err == nil && len(strings.TrimSpace(string(out))) > 0

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

	if isOpenWrt() {

		initScript := "/etc/init.d/openflux"

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

	if isOpenWrt() {

		initScript := "/etc/init.d/openflux"

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

	if isOpenWrt() {

		initScript := "/etc/init.d/openflux"

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

	if isOpenWrt() {

		initScript := "/etc/init.d/openflux"

		action := "disable"

		if on {

			action = "enable"

		}

		return exec.Command(initScript, action).Run()

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

		_ = os.Remove("/opt/var/log/openflux.log")

		return nil

	}

	if isOpenWrt() {

		_ = c.Stop(dir)

		_ = exec.Command("/etc/init.d/openflux", "disable").Run()

		_ = os.Remove("/etc/init.d/openflux")

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

