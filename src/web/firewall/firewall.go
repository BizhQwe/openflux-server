package firewall

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var (
	openedMu    sync.Mutex
	openedPorts = make(map[int]bool)
)

// OpenPorts ensures the given TCP ports are allowed in the system firewall
// (UFW, firewalld, iptables on Linux; Windows Defender Firewall on Windows)
// and attempts router port forwarding via UPnP.
func OpenPorts(ports []int, appName, appPath string) error {
	openedMu.Lock()
	defer openedMu.Unlock()

	var firstErr error
	for _, port := range ports {
		if port <= 0 || port > 65535 {
			continue
		}
		if err := openOSPort(port, appName, appPath); err != nil && firstErr == nil {
			firstErr = err
		}
		openedPorts[port] = true
	}

	// Trigger asynchronous UPnP router port forwarding with timeout
	go func(portsToMap []int) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ForwardPortsUPnP(ctx, portsToMap, appName)
	}(append([]int(nil), ports...))

	return firstErr
}

// OpenPort allows a single TCP port in the OS firewall and UPnP.
func OpenPort(port int, appName string) error {
	return OpenPorts([]int{port}, appName, "")
}

// ClosePorts removes the specified TCP ports from the system firewall.
func ClosePorts(ports []int, appName string) error {
	openedMu.Lock()
	defer openedMu.Unlock()

	var firstErr error
	for _, port := range ports {
		if port <= 0 || port > 65535 {
			continue
		}
		if err := closeOSPort(port, appName); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(openedPorts, port)
	}
	return firstErr
}

// RemoveAll cleans up all OpenFlux firewall rules and application rules.
func RemoveAll(appName, appPath string) error {
	openedMu.Lock()
	defer openedMu.Unlock()

	var ports []int
	for port := range openedPorts {
		ports = append(ports, port)
	}
	// Common OpenFlux ports to ensure clean uninstall
	ports = append(ports, 5000, 80, 443)

	return removeAllOSRules(ports, appName, appPath)
}

func formatRuleName(port int, appName string) string {
	if appName == "" {
		appName = "OpenFlux"
	}
	return fmt.Sprintf("OpenFlux %s Port %d", appName, port)
}
