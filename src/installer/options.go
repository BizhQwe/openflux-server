package main

import (
	"errors"
	"net"
	"strconv"
	"strings"

	"openflux.local/server/installer/internal/terminal"
	"openflux.local/server/panel/configuration"
)

type installOptions struct {
	Listen, Mode, Domain, Language                     string
	Port                                               int
	PortSet, ExplicitPort                              bool
	NoService, Silent, LanguageSet, ModeSet, DomainSet bool
	Autostart                                          bool
	AutostartSet                                       bool
	OpenPort                                           bool
	OpenPortSet                                        bool
}

func publishMode(s configuration.Settings) string {
	if s.LocaltunnelEnabled {
		return "localtunnel"
	}
	if s.DomainEnabled {
		return "domain"
	}
	if configuration.LANEnabled(s) {
		return "lan"
	}
	return "localhost"
}
func (o *installOptions) wizard(u *terminal.UI, previous configuration.Settings, fresh bool) error {
	if o.Language == "" {
		o.Language = previous.Language
	}
	if o.Language == "" {
		o.Language = "en"
	}
	if o.Language != "en" && o.Language != "ru" {
		return errors.New("--lang must be ru or en")
	}
	u.Language = o.Language
	interactive := u.Interactive && !o.Silent
	if interactive && !o.LanguageSet {
		fmtDefault := 0
		if o.Language == "ru" {
			fmtDefault = 1
		}
		choice := u.Choice("Language / Язык", []string{"English", "Русский"}, fmtDefault)
		o.Language = []string{"en", "ru"}[choice]
		u.Language = o.Language
		o.LanguageSet = true
	}
	if interactive && !o.ModeSet {
		modes := []string{"localhost", "lan", "localtunnel", "domain"}
		defaultIndex := 3
		if !fresh {
			for i, mode := range modes {
				if mode == publishMode(previous) {
					defaultIndex = i
				}
			}
		}
		options := []string{
			u.Text("Только на этом ПК (localhost)", "Only on this PC (localhost)"),
			u.Text("Через локальную сеть (LAN / Wi-Fi)", "Local network (LAN / Wi-Fi)"),
			u.Text("Через Localtunnel (без публичного IP)", "Via Localtunnel (no public IP required)"),
			u.Text("Прямое подключение (домен / публичный IP)", "Direct connection (domain / public IP)"),
		}
		o.Mode = modes[u.Choice(u.Text("Сетевое размещение", "Network accessibility"), options, defaultIndex)]
		o.ModeSet = true
	}
	if interactive && o.Mode == "domain" && !o.DomainSet {
		o.Domain = u.AskText(u.Text("Домен без схемы — прямой доступ; полный URL — готовый reverse proxy (Enter — без домена)", "Hostname for direct access, full URL for an existing reverse proxy (Enter for no domain)"), previous.Domain)
		o.DomainSet = true
	}
	if interactive && !o.PortSet && o.Listen == "" {
		defaultPort := 5000
		if o.Port > 0 && o.Port <= 65535 {
			defaultPort = o.Port
		} else if _, p, err := net.SplitHostPort(previous.Listen); err == nil {
			if n, err := strconv.Atoi(p); err == nil && n > 0 && n <= 65535 {
				defaultPort = n
			}
		}
		for {
			val := u.AskText(u.Text("Порт веб-панели", "Web panel port"), strconv.Itoa(defaultPort))
			parsed, err := strconv.Atoi(strings.TrimSpace(val))
			if err == nil && parsed >= 1 && parsed <= 65535 {
				o.Port = parsed
				o.PortSet = true
				if val != strconv.Itoa(defaultPort) {
					o.ExplicitPort = true
				}
				break
			}
			u.Error(u.Text("Порт должен быть числом от 1 до 65535", "Port must be a number between 1 and 65535"))
			if !u.Interactive {
				o.Port = defaultPort
				o.PortSet = true
				break
			}
		}
	}
	if interactive && !o.NoService && !o.AutostartSet {
		o.Autostart = u.YesNo(u.Text("Включить автозапуск службы при загрузке системы?", "Enable server autostart on system boot?"), true)
	}
	if interactive && !o.OpenPortSet {
		o.OpenPort = u.YesNo(u.Text("Открыть порт панели в брандмауэре (firewall)?", "Open panel port in system firewall?"), true)
		o.OpenPortSet = true
	}
	return nil
}
func (o installOptions) apply(s *configuration.Settings) error {
	if o.ModeSet {
		s.LocaltunnelEnabled = false
		s.DomainEnabled = false
		switch strings.ToLower(o.Mode) {
		case "domain", "direct":
			configuration.SetLAN(s, false)
			configuration.SetPublicListen(s, true)
			s.DomainEnabled = strings.TrimSpace(s.Domain) != ""
		case "lan", "local":
			configuration.SetLAN(s, true)
		case "localtunnel":
			s.LocaltunnelEnabled = true
			configuration.SetLAN(s, true)
		case "localhost":
			configuration.SetLAN(s, false)
		default:
			return errors.New("--mode must be domain, localtunnel, lan or localhost")
		}
	}
	// An explicit listen address takes priority over the wizard's host choice.
	if o.Listen != "" {
		s.Listen = o.Listen
	}
	if o.Port != 0 {
		if o.Port < 1 || o.Port > 65535 {
			return errors.New("--port must be between 1 and 65535")
		}
		host, _, e := net.SplitHostPort(s.Listen)
		if e != nil {
			return e
		}
		s.Listen = net.JoinHostPort(host, strconv.Itoa(o.Port))
	}
	if o.DomainSet {
		s.Domain = strings.TrimSpace(o.Domain)
		s.DomainEnabled = s.Domain != ""
		if s.DomainEnabled {
			_, e := configuration.DomainAddress(s.Domain)
			if e != nil {
				return e
			}
		}
	}
	if o.LanguageSet {
		s.Language = o.Language
	}
	return configuration.ValidateSettings(*s)
}
