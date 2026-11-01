//go:build !windows && !linux

package firewall

func openOSPort(port int, appName, appPath string) error {
	return nil
}

func closeOSPort(port int, appName string) error {
	return nil
}

func removeAllOSRules(ports []int, appName, appPath string) error {
	return nil
}
