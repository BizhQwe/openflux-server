package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
)

func defaultInstallDir() string {
	base := os.Getenv("ProgramFiles")
	if base == "" {
		base = `C:\Program Files`
	}
	return filepath.Join(base, "OpenFlux Server")
}
func hideProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
func protectData(dir string) error {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return e
	}
	path := `"` + dir + `"`
	code := `icacls.exe ` + path + ` /inheritance:r /grant:r "SYSTEM:(OI)(CI)F" /grant:r "*S-1-5-32-544:(OI)(CI)F"`
	_, e := command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", code)
	return e
}
func command(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	hideProcess(cmd)
	return cmd.CombinedOutput()
}
func preflightService() error {
	code := `$identity=[Security.Principal.WindowsIdentity]::GetCurrent();$principal=New-Object Security.Principal.WindowsPrincipal($identity);if(-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)){exit 1}`
	if _, e := command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", code); e != nil {
		return errors.New("run this installer as Administrator, or use --no-service --dir <directory>")
	}
	return nil
}
func stopService() error {
	for _, task := range []string{"OpenFluxServer", "OpenFluxZenServer"} {
		if _, e := command("schtasks.exe", "/Query", "/TN", task); e == nil {
			_, _ = command("schtasks.exe", "/End", "/TN", task)
		}
	}
	time.Sleep(time.Second)
	return nil
}
func taskXML(dir string) string {
	escape := func(value string) string {
		var b strings.Builder
		_ = xml.EscapeText(&b, []byte(value))
		return b.String()
	}
	return `<?xml version="1.0" encoding="UTF-16"?><Task version="1.4" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task"><RegistrationInfo><Description>OpenFlux Server</Description></RegistrationInfo><Triggers><BootTrigger><Enabled>true</Enabled><Delay>PT10S</Delay></BootTrigger></Triggers><Principals><Principal id="System"><UserId>S-1-5-18</UserId><RunLevel>HighestAvailable</RunLevel></Principal></Principals><Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries><StopIfGoingOnBatteries>false</StopIfGoingOnBatteries><StartWhenAvailable>true</StartWhenAvailable><ExecutionTimeLimit>PT0S</ExecutionTimeLimit><RestartOnFailure><Interval>PT1M</Interval><Count>3</Count></RestartOnFailure></Settings><Actions Context="System"><Exec><Command>` + escape(filepath.Join(dir, "openflux-server-web.exe")) + `</Command><Arguments>` + escape(`--data-dir "`+filepath.Join(dir, "data")+`"`) + `</Arguments><WorkingDirectory>` + escape(dir) + `</WorkingDirectory></Exec></Actions></Task>`
}
func registerService(dir string, autostart bool) error {
	temp, e := os.CreateTemp("", "openflux-task-*.xml")
	if e != nil {
		return e
	}
	path := temp.Name()
	_ = temp.Close()
	defer os.Remove(path)
	units := utf16.Encode([]rune(taskXML(dir)))
	data := make([]byte, 2+len(units)*2)
	data[0] = 0xff
	data[1] = 0xfe
	for i, v := range units {
		data[2+i*2] = byte(v)
		data[3+i*2] = byte(v >> 8)
	}
	if e = os.WriteFile(path, data, 0644); e != nil {
		return e
	}
	_, _ = command("schtasks.exe", "/Delete", "/TN", "OpenFluxZenServer", "/F")
	if b, e := command("schtasks.exe", "/Create", "/TN", "OpenFluxServer", "/XML", path, "/RU", "SYSTEM", "/F"); e != nil {
		return fmt.Errorf("schtasks create: %s: %w", b, e)
	}
	boot := "DISABLE"
	if autostart {
		boot = "ENABLE"
	}
	if b, e := command("schtasks.exe", "/Change", "/TN", "OpenFluxServer", "/" + boot); e != nil {
		return fmt.Errorf("schtasks change autostart: %s: %w", b, e)
	}
	return nil
}

var forceEntware bool
func isEntware() bool { return false }
func isOpenWrt() bool { return false }
