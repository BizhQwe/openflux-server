package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"openflux.local/server/panel/configuration"
)

func aliasTarget(alias string) (string, error) {
	value, e := os.Readlink(alias)
	if e != nil {
		return "", e
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(filepath.Dir(alias), value)
	}
	return filepath.Clean(value), nil
}

func isBinaryOpenFlux(path string) bool {
	if f, err := os.Open(path); err == nil {
		buf := make([]byte, 128*1024)
		n, _ := f.Read(buf)
		_ = f.Close()
		if strings.Contains(strings.ToLower(string(buf[:n])), "openflux") {
			return true
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "version")
	if out, err := cmd.CombinedOutput(); err == nil {
		if strings.Contains(strings.ToLower(string(out)), "openflux") {
			return true
		}
	}
	return false
}

func isOwnOrUpgradable(alias, target, legacyTarget string) bool {
	info, err := os.Lstat(alias)
	if os.IsNotExist(err) {
		return true
	}
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		old, err := aliasTarget(alias)
		if err != nil {
			return true
		}
		if configuration.SamePath(old, target) || configuration.SamePath(old, legacyTarget) {
			return true
		}
		if _, statErr := os.Stat(old); os.IsNotExist(statErr) {
			return true
		}
		if strings.Contains(strings.ToLower(old), "openflux") {
			return true
		}
		if isBinaryOpenFlux(old) {
			return true
		}
		return false
	}
	return isBinaryOpenFlux(alias)
}

func registerAliases(target, legacyTarget string, aliases []string) error {
	if info, e := os.Stat(target); e != nil {
		return e
	} else if !info.Mode().IsRegular() {
		return fmt.Errorf("CLI executable is not a file: %s", target)
	}
	// Validate every destination before changing any command owned by another app.
	for _, alias := range aliases {
		if !isOwnOrUpgradable(alias, target, legacyTarget) {
			return fmt.Errorf("команда %s принадлежит другой программе", alias)
		}
	}
	var registered int
	var lastErr error
	for _, alias := range aliases {
		old, e := aliasTarget(alias)
		if e == nil && configuration.SamePath(old, target) {
			registered++
			continue
		}
		if e = os.MkdirAll(filepath.Dir(alias), 0755); e != nil {
			lastErr = e
			continue
		}
		file, e := os.CreateTemp(filepath.Dir(alias), ".openflux-alias-*")
		if e != nil {
			lastErr = e
			continue
		}
		temporary := file.Name()
		_ = file.Close()
		_ = os.Remove(temporary)
		if e = os.Symlink(target, temporary); e == nil {
			if e = os.Rename(temporary, alias); e != nil {
				_ = os.Remove(alias)
				e = os.Symlink(target, alias)
			}
		}
		_ = os.Remove(temporary)
		if e != nil {
			lastErr = e
			continue
		}
		registered++
	}
	if registered == 0 && lastErr != nil {
		return lastErr
	}
	return nil
}

func removeAliases(target, legacyTarget string, aliases []string) error {
	for _, alias := range aliases {
		old, e := aliasTarget(alias)
		if e != nil {
			continue
		}
		if configuration.SamePath(old, target) || configuration.SamePath(old, legacyTarget) || strings.Contains(strings.ToLower(old), "openflux") {
			if e = os.Remove(alias); e != nil {
				return e
			}
		}
	}
	return nil
}
