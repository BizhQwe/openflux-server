package firewall

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestFormatRuleName(t *testing.T) {
	name := formatRuleName(5000, "Panel")
	if !strings.Contains(name, "5000") || !strings.Contains(name, "Panel") {
		t.Fatalf("unexpected rule name: %s", name)
	}

	defaultName := formatRuleName(80, "")
	if !strings.Contains(defaultName, "80") || !strings.Contains(defaultName, "OpenFlux") {
		t.Fatalf("unexpected default rule name: %s", defaultName)
	}
}

func TestFindWANService(t *testing.T) {
	sampleXML := `<?xml version="1.0"?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <deviceType>urn:schemas-upnp-org:device:InternetGatewayDevice:1</deviceType>
    <deviceList>
      <device>
        <deviceType>urn:schemas-upnp-org:device:WANDevice:1</deviceType>
        <deviceList>
          <device>
            <deviceType>urn:schemas-upnp-org:device:WANConnectionDevice:1</deviceType>
            <serviceList>
              <service>
                <serviceType>urn:schemas-upnp-org:service:WANIPConnection:1</serviceType>
                <controlURL>/ctl/IPConn</controlURL>
              </service>
            </serviceList>
          </device>
        </deviceList>
      </device>
    </deviceList>
  </device>
</root>`

	var root rootXML
	if err := xml.Unmarshal([]byte(sampleXML), &root); err != nil {
		t.Fatalf("XML unmarshal failed: %v", err)
	}

	srv := findWANService(&root.Device)
	if srv == nil {
		t.Fatal("expected WAN service to be found")
	}
	if srv.ServiceType != "urn:schemas-upnp-org:service:WANIPConnection:1" {
		t.Fatalf("unexpected service type: %s", srv.ServiceType)
	}
	if srv.ControlURL != "/ctl/IPConn" {
		t.Fatalf("unexpected control URL: %s", srv.ControlURL)
	}
}
