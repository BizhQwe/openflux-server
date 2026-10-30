package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSystemdWorkingDirectoryIsAnAbsoluteScalar(t *testing.T) {
	for _, dir := range []string{"/opt/openflux-server", "/opt/OpenFlux Server", "/opt/100% $HOME"} {
		unit, e := systemdUnit(dir)
		if e != nil {
			t.Fatal(e)
		}
		for _, line := range strings.Split(unit, "\n") {
			if strings.HasPrefix(line, "WorkingDirectory=") {
				value := strings.TrimPrefix(line, "WorkingDirectory=")
				if !strings.HasPrefix(value, "/") || value != strings.ReplaceAll(dir, "%", "%%") {
					t.Fatalf("systemd would reject or change the working directory: %s", line)
				}
			}
		}
		if !strings.Contains(unit, "openflux-server-web\" --data-dir \"") {
			t.Fatal("wrong service executable or arguments")
		}
		if strings.Contains(dir, "%") && (!strings.Contains(unit, "100%% $HOME/openflux-server-web") || !strings.Contains(unit, "ExecStart=:")) {
			t.Fatal("ExecStart interprets specifiers/environment")
		}
	}
	for _, dir := range []string{"relative", "/opt/test\nExecStart=/bin/false", "/opt/test ", "/opt/test\x00"} {
		if _, e := systemdUnit(dir); e == nil {
			t.Fatalf("accepted invalid path %q", dir)
		}
	}
}

func TestSystemdAnalyzeVerify(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("systemd-analyze runs on Linux")
	}
	tool, e := exec.LookPath("systemd-analyze")
	if e != nil {
		t.Skip("systemd-analyze is unavailable")
	}
	dir := filepath.Join(t.TempDir(), "OpenFlux Server")
	if e = os.Mkdir(dir, 0755); e != nil {
		t.Fatal(e)
	}
	// A real executable lets the verifier validate ExecStart without a built panel.
	if e = os.Symlink("/bin/true", filepath.Join(dir, "openflux-server-web")); e != nil {
		t.Fatal(e)
	}
	unit, e := systemdUnit(dir)
	if e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(t.TempDir(), "openflux-server.service")
	if e = os.WriteFile(file, []byte(unit), 0644); e != nil {
		t.Fatal(e)
	}
	if output, err := exec.Command(tool, "verify", file).CombinedOutput(); err != nil {
		t.Fatalf("systemd rejects the generated unit: %s: %v", output, err)
	}
}
