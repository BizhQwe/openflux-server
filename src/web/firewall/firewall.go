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
// Port opening is executed asynchronously in the background so it never blocks
// the caller or delays network operations.
func OpenPorts(ports []int, appName, appPath string) error {
	openedMu.Lock()
	var valid []int
	for _, port := range ports {
		if port > 0 && port <= 65535 {
			openedPorts[port] = true
			valid = append(valid, port)
		}
	}
	openedMu.Unlock()

	if len(valid) == 0 {
		return nil
	}

	go func(portsToOpen []int) {
		for _, port := range portsToOpen {
			_ = openOSPort(port, appName, appPath)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = ForwardPortsUPnP(ctx, portsToOpen, appName)
	}(valid)

	return nil
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

// ClosePort removes a single TCP port from the OS firewall and application rules.
func ClosePort(port int, appName, appPath string) error {
	openedMu.Lock()
	delete(openedPorts, port)
	openedMu.Unlock()
	return removeAllOSRules([]int{port}, appName, appPath)
}

// RemoveAll cleans up all OpenFlux firewall rules and application rules.
func RemoveAll(appName, appPath string) error {
	openedMu.Lock()
	defer openedMu.Unlock()

	var ports []int
	for port := range openedPorts {
		ports = append(ports, port)
	}

	return removeAllOSRules(ports, appName, appPath)
}

func formatRuleName(port int, appName string) string {
	if appName == "" {
		appName = "OpenFlux"
	}
	return fmt.Sprintf("OpenFlux %s Port %d", appName, port)
}
