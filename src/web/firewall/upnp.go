package firewall

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type upnpService struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

type upnpDevice struct {
	DeviceType string        `xml:"deviceType"`
	Services   []upnpService `xml:"serviceList>service"`
	Devices    []upnpDevice  `xml:"deviceList>device"`
}

type rootXML struct {
	XMLName xml.Name   `xml:"root"`
	Device  upnpDevice `xml:"device"`
}

func findWANService(dev *upnpDevice) *upnpService {
	for i := range dev.Services {
		st := dev.Services[i].ServiceType
		if strings.Contains(st, ":WANIPConnection:") || strings.Contains(st, ":WANPPPConnection:") {
			return &dev.Services[i]
		}
	}
	for i := range dev.Devices {
		if s := findWANService(&dev.Devices[i]); s != nil {
			return s
		}
	}
	return nil
}

// ForwardPortsUPnP attempts to map external TCP ports to this local machine via UPnP IGD.
func ForwardPortsUPnP(ctx context.Context, ports []int, desc string) error {
	if len(ports) == 0 {
		return nil
	}
	if desc == "" {
		desc = "OpenFlux Server"
	}

	controlURL, serviceType, routerAddr, err := discoverUPnP(ctx)
	if err != nil {
		return err
	}

	localIP, err := getLocalIPForTarget(routerAddr)
	if err != nil {
		return err
	}

	for _, port := range ports {
		if port <= 0 || port > 65535 {
			continue
		}
		_ = addPortMapping(ctx, controlURL, serviceType, localIP, port, desc)
	}

	return nil
}

func discoverUPnP(ctx context.Context) (controlURL, serviceType, routerAddr string, err error) {
	ssdpAddr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return "", "", "", err
	}

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return "", "", "", err
	}
	defer conn.Close()

	msearch := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n\r\n"

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err = conn.WriteToUDP([]byte(msearch), ssdpAddr); err != nil {
		return "", "", "", err
	}

	buf := make([]byte, 2048)
	var location string
	for {
		select {
		case <-ctx.Done():
			return "", "", "", ctx.Err()
		default:
		}

		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			break
		}

		resp := string(buf[:n])
		for _, line := range strings.Split(resp, "\r\n") {
			lower := strings.ToLower(line)
			if strings.HasPrefix(lower, "location:") {
				location = strings.TrimSpace(line[len("location:"):])
				routerAddr = src.String()
				break
			}
		}
		if location != "" {
			break
		}
	}

	if location == "" {
		return "", "", "", fmt.Errorf("no UPnP IGD location found")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", location, nil)
	if err != nil {
		return "", "", "", err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", "", "", err
	}

	var root rootXML
	if err = xml.Unmarshal(body, &root); err != nil {
		return "", "", "", err
	}

	srv := findWANService(&root.Device)
	if srv == nil {
		return "", "", "", fmt.Errorf("no WAN IP or PPP connection service found in UPnP XML")
	}

	locURL, err := url.Parse(location)
	if err != nil {
		return "", "", "", err
	}

	cURL, err := url.Parse(srv.ControlURL)
	if err != nil {
		return "", "", "", err
	}

	finalURL := locURL.ResolveReference(cURL).String()
	return finalURL, srv.ServiceType, locURL.Host, nil
}

func getLocalIPForTarget(targetHostPort string) (string, error) {
	conn, err := net.DialTimeout("udp", targetHostPort, 2*time.Second)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

func addPortMapping(ctx context.Context, controlURL, serviceType, localIP string, port int, desc string) error {
	soapBody := fmt.Sprintf(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
<s:Body>
<u:AddPortMapping xmlns:u="%s">
<NewRemoteHost></NewRemoteHost>
<NewExternalPort>%d</NewExternalPort>
<NewProtocol>TCP</NewProtocol>
<NewInternalPort>%d</NewInternalPort>
<NewInternalClient>%s</NewInternalClient>
<NewEnabled>1</NewEnabled>
<NewPortMappingDescription>%s</NewPortMappingDescription>
<NewLeaseDuration>0</NewLeaseDuration>
</u:AddPortMapping>
</s:Body>
</s:Envelope>`, serviceType, port, port, localIP, desc)

	req, err := http.NewRequestWithContext(ctx, "POST", controlURL, bytes.NewBufferString(soapBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#AddPortMapping"`, serviceType))

	client := &http.Client{Timeout: 3 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
