package platform

import (
	"openflux.local/server/panel/configuration"
	"os"
)

func RemoveInstallation(dir string) error {
	if _, e := configuration.ReadInstallation(dir); e != nil {
		return e
	}
	return os.RemoveAll(dir)
}
