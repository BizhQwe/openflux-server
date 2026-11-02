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

	// 2. Try firewalld if active (without --reload which wipes other programs' runtime rules)
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		if runCmd("firewall-cmd", "--state") == nil {
			attempted = true
			_ = runCmd("firewall-cmd", fmt.Sprintf("--add-port=%s", tcpPort))
			_ = runCmd("firewall-cmd", "--permanent", fmt.Sprintf("--add-port=%s", tcpPort))
		}
	}

	// 3. Try iptables with comment tag so we never delete other apps' rules
	if _, err := exec.LookPath("iptables"); err == nil {
		attempted = true
		if runCmd("iptables", "-C", "INPUT", "-p", "tcp", "--dport", portStr, "-m", "comment", "--comment", comment, "-j", "ACCEPT") != nil {
			if runCmd("iptables", "-I", "INPUT", "1", "-p", "tcp", "--dport", portStr, "-m", "comment", "--comment", comment, "-j", "ACCEPT") != nil {
				if runCmd("iptables", "-C", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT") != nil {
					_ = runCmd("iptables", "-I", "INPUT", "1", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT")
				}
			}
		}
	}

	// 4. Try ip6tables with comment tag
	if _, err := exec.LookPath("ip6tables"); err == nil {
		attempted = true
		if runCmd("ip6tables", "-C", "INPUT", "-p", "tcp", "--dport", portStr, "-m", "comment", "--comment", comment, "-j", "ACCEPT") != nil {
			if runCmd("ip6tables", "-I", "INPUT", "1", "-p", "tcp", "--dport", portStr, "-m", "comment", "--comment", comment, "-j", "ACCEPT") != nil {
				if runCmd("ip6tables", "-C", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT") != nil {
					_ = runCmd("ip6tables", "-I", "INPUT", "1", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT")
				}
			}
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
	comment := fmt.Sprintf("OpenFlux %s", appName)

	if _, err := exec.LookPath("ufw"); err == nil {
		_ = runCmd("ufw", "delete", "allow", tcpPort)
	}

	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		if runCmd("firewall-cmd", "--state") == nil {
			_ = runCmd("firewall-cmd", fmt.Sprintf("--remove-port=%s", tcpPort))
			_ = runCmd("firewall-cmd", "--permanent", fmt.Sprintf("--remove-port=%s", tcpPort))
		}
	}

	if _, err := exec.LookPath("iptables"); err == nil {
		if runCmd("iptables", "-D", "INPUT", "-p", "tcp", "--dport", portStr, "-m", "comment", "--comment", comment, "-j", "ACCEPT") != nil {
			_ = runCmd("iptables", "-D", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT")
		}
	}

	if _, err := exec.LookPath("ip6tables"); err == nil {
		if runCmd("ip6tables", "-D", "INPUT", "-p", "tcp", "--dport", portStr, "-m", "comment", "--comment", comment, "-j", "ACCEPT") != nil {
			_ = runCmd("ip6tables", "-D", "INPUT", "-p", "tcp", "--dport", portStr, "-j", "ACCEPT")
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
