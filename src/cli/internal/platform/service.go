package platform

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"openflux.local/server/panel/configuration"
)

const ServiceName = "openflux-server.service"
const TaskName = "OpenFluxServer"

type Status struct {
	Registered bool   `json:"registered"`
	Running    bool   `json:"running"`
	Enabled    bool   `json:"enabled"`
	Executable string `json:"executable"`
}
type Service interface {
	Status(string) (Status, error)
	Start(string) error
	Stop(string) error
	Enable(string, bool) error
	Remove(string) error
}
type Controller struct {
	Run func(string, ...string) ([]byte, error)
}

func New() *Controller { return &Controller{Run: runCommand} }
func runCommand(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	Hide(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s: %s: %w", name, out, err)
	}
	return out, nil
}
func (c *Controller) owned(dir string) (Status, error) {
	s, e := c.Status(dir)
	if e != nil {
		return s, e
	}
	if !s.Registered {
		if ConsoleSupportsCyrillic() {
			return s, errors.New("служба не зарегистрирована; запустите цельный установщик без --no-service")
		}
		return s, errors.New("service is not registered; run full installer without --no-service")
	}
	if !configuration.SamePath(s.Executable, PanelPath(dir)) {
		if ConsoleSupportsCyrillic() {
			return s, errors.New("служба принадлежит другой установке; укажите её каталог через --dir")
		}
		return s, errors.New("service belongs to another installation; specify its directory with --dir")
	}
	return s, nil
}
func (c *Controller) wait(dir string, running bool) error {
	for i := 0; i < 100; i++ {
		s, e := c.Status(dir)
		if e != nil {
			return e
		}
		if s.Running == running {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	if ConsoleSupportsCyrillic() {
		return errors.New("служба не перешла в ожидаемое состояние за 25 секунд")
	}
	return errors.New("service did not transition to expected state within 25 seconds")
}
