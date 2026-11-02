package platform

import (
	"os"
	"path/filepath"
)

const AliasPath = "/usr/local/bin/openflux"
const FallbackAliasPath = "/usr/bin/openflux"
const EntwareAliasPath = "/opt/bin/openflux"

func RegisterPath(dir string) (bool, error) {
	if e := os.Chmod(dir, 0755); e != nil {
		return false, e
	}
	aliases := []string{AliasPath, FallbackAliasPath}
	if fi, err := os.Stat("/opt/bin"); err == nil && fi.IsDir() {
		aliases = append(aliases, EntwareAliasPath)
	}
	e := registerAliases(filepath.Join(dir, "openflux-server-cli"), filepath.Join(dir, "openflux"), aliases)
	return e == nil, e
}

func RemovePath(dir string, owned bool) error {
	if !owned {
		return nil
	}
	aliases := []string{AliasPath, FallbackAliasPath, EntwareAliasPath}
	return removeAliases(filepath.Join(dir, "openflux-server-cli"), filepath.Join(dir, "openflux"), aliases)
}
