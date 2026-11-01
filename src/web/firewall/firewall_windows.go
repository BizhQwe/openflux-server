//go:build windows

package firewall

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
)

func hideProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}

func runNetsh(args ...string) error {
	cmd := exec.Command("netsh.exe", args...)
	hideProcess(cmd)
	return cmd.Run()
}

func openOSPort(port int, appName, appPath string) error {
	ruleName := formatRuleName(port, appName)
	portStr := strconv.Itoa(port)

	// Remove old rule if present to avoid duplication
	_ = runNetsh("advfirewall", "firewall", "delete", "rule", "name="+ruleName)

	// Add inbound TCP allow rule
	err := runNetsh(
		"advfirewall", "firewall", "add", "rule",
		"name="+ruleName,
		"dir=in",
		"action=allow",
		"protocol=TCP",
		"localport="+portStr,
		"profile=any",
	)

	// If application path is supplied, also whitelist the executable directly
	if appPath != "" {
		appRule := fmt.Sprintf("OpenFlux %s App", appName)
		_ = runNetsh("advfirewall", "firewall", "delete", "rule", "name="+appRule)
		_ = runNetsh(
			"advfirewall", "firewall", "add", "rule",
			"name="+appRule,
			"dir=in",
			"action=allow",
			"program="+appPath,
			"enable=yes",
			"profile=any",
		)
	}

	return err
}

func closeOSPort(port int, appName string) error {
	ruleName := formatRuleName(port, appName)
	return runNetsh("advfirewall", "firewall", "delete", "rule", "name="+ruleName)
}

func removeAllOSRules(ports []int, appName, appPath string) error {
	for _, port := range ports {
		_ = closeOSPort(port, appName)
	}
	appRule := fmt.Sprintf("OpenFlux %s App", appName)
	_ = runNetsh("advfirewall", "firewall", "delete", "rule", "name="+appRule)
	return nil
}
