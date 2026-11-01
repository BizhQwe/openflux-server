//go:build linux

package firewall

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

func openOSPort(port int, appName, appPath string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	portStr := strconv.Itoa(port)
	tcpPort := fmt.Sprintf("%d/tcp", port)
	comment := fmt.Sprintf("OpenFlux %s", appName)

	var attempted bool

	// 1. Try UFW if available
	if _, err := exec.LookPath("ufw"); err == nil {
		attempted = true
		_ = runCmd("ufw", "allow", tcpPort, "comment", comment)
	}

	// 2. Try firewalld if active
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		if runCmd("firewall-cmd", "--state") == nil {
			attempted = true
			_ = runCmd("firewall-cmd", "--permanent", fmt.Sprintf("--add-port=%s", tcpPort))
			_ = runCmd("firewall-cmd", "--reload")
		}
	}

	// 3. Try iptables
	if _, err := exec.LookPath("iptables"); err == nil {
		attempted = true
		if runCmd("iptables", "-C", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT") != nil {
			_ = runCmd("iptables", "-I", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT")
		}
	}

	// 4. Try ip6tables
	if _, err := exec.LookPath("ip6tables"); err == nil {
		if runCmd("ip6tables", "-C", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT") != nil {
			_ = runCmd("ip6tables", "-I", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT")
		}
	}

	if !attempted {
		return fmt.Errorf("no supported firewall utility found (ufw, firewalld, iptables)")
	}
	return nil
}

func closeOSPort(port int, appName string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	portStr := strconv.Itoa(port)
	tcpPort := fmt.Sprintf("%d/tcp", port)

	if _, err := exec.LookPath("ufw"); err == nil {
		_ = runCmd("ufw", "delete", "allow", tcpPort)
	}

	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		if runCmd("firewall-cmd", "--state") == nil {
			_ = runCmd("firewall-cmd", "--permanent", fmt.Sprintf("--remove-port=%s", tcpPort))
			_ = runCmd("firewall-cmd", "--reload")
		}
	}

	if _, err := exec.LookPath("iptables"); err == nil {
		for runCmd("iptables", "-D", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT") == nil {
		}
	}

	if _, err := exec.LookPath("ip6tables"); err == nil {
		for runCmd("ip6tables", "-D", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT") == nil {
		}
	}

	return nil
}

func removeAllOSRules(ports []int, appName, appPath string) error {
	for _, port := range ports {
		_ = closeOSPort(port, appName)
	}
	return nil
}
