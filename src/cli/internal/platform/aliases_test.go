package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandAliasesUpgradeAndRefuseForeignCommands(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "openflux-server-cli")
	legacy := filepath.Join(dir, "openflux")
	if e := os.WriteFile(target, []byte("test binary"), 0755); e != nil {
		t.Fatal(e)
	}
	primary := filepath.Join(dir, "local-bin", "openflux")
	fallback := filepath.Join(dir, "bin", "openflux")
	if e := os.MkdirAll(filepath.Dir(primary), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(legacy, primary); e != nil {
		t.Skipf("local test symlinks unavailable: %v", e)
	}
	if e := registerAliases(target, legacy, []string{primary, fallback}); e != nil {
		t.Fatal(e)
	}
	for _, alias := range []string{primary, fallback} {
		info, e := os.Stat(alias)
		real, _ := os.Stat(target)
		if e != nil || !os.SameFile(info, real) {
			t.Fatalf("broken alias: %s", alias)
		}
	}
	if e := registerAliases(target, legacy, []string{primary, fallback}); e != nil {
		t.Fatal("reinstallation broke the command")
	}
	if e := os.Remove(fallback); e != nil {
		t.Fatal(e)
	}
	foreign := []byte("another application's command")
	if e := os.WriteFile(fallback, foreign, 0755); e != nil {
		t.Fatal(e)
	}
	if e := registerAliases(target, legacy, []string{primary, fallback}); e == nil {
		t.Fatal("replaced a foreign command")
	}
	if e := removeAliases(target, legacy, []string{primary, fallback}); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Lstat(primary); !os.IsNotExist(e) {
		t.Fatal("own alias remains")
	}
	if contents, _ := os.ReadFile(fallback); string(contents) != string(foreign) {
		t.Fatal("foreign command changed")
	}
}

func TestCommandAliasesUpgradeExistingOpenFlux(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "openflux-server-cli")
	legacy := filepath.Join(dir, "openflux")
	if e := os.WriteFile(target, []byte("new openflux binary"), 0755); e != nil {
		t.Fatal(e)
	}
	primary := filepath.Join(dir, "local-bin", "openflux")
	fallback := filepath.Join(dir, "bin", "openflux")
	if e := os.MkdirAll(filepath.Dir(primary), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.MkdirAll(filepath.Dir(fallback), 0755); e != nil {
		t.Fatal(e)
	}

	// Case 1: Primary is a symlink to an old openflux install path
	oldTarget := filepath.Join(dir, "opt", "openflux-zen-server", "openflux-zen-server-cli")
	_ = os.MkdirAll(filepath.Dir(oldTarget), 0755)
	_ = os.WriteFile(oldTarget, []byte("old openflux"), 0755)
	if e := os.Symlink(oldTarget, primary); e != nil {
		t.Skipf("local test symlinks unavailable: %v", e)
	}

	// Case 2: Fallback is an existing openflux binary file
	if e := os.WriteFile(fallback, []byte("previous openflux binary file"), 0755); e != nil {
		t.Fatal(e)
	}

	// Should successfully upgrade both aliases without throwing "принадлежит другой программе"
	if e := registerAliases(target, legacy, []string{primary, fallback}); e != nil {
		t.Fatalf("failed to upgrade existing openflux aliases: %v", e)
	}

	for _, alias := range []string{primary, fallback} {
		info, e := os.Stat(alias)
		real, _ := os.Stat(target)
		if e != nil || !os.SameFile(info, real) {
			t.Fatalf("alias %s does not point to new target", alias)
		}
	}
}
