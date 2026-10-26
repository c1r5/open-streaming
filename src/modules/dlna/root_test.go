package dlna

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	dms "github.com/anacrolix/dms/dlna/dms"
	"github.com/anacrolix/dms/upnpav"
)

func TestBrowseDirectChildrenReachesCallback(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "192.168.1.100", "::1"} {
		if !clientAllowed(net.ParseIP(ip)) {
			t.Fatalf("allow list rejects %s", ip)
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &dms.Server{
		HTTPConn:       ln,
		FriendlyName:   "local-tv-test",
		RootObjectPath: t.TempDir(),
		Interfaces:     []net.Interface{},
		AllowedIpNets:  allowAllIPNets(),
		OnBrowseDirectChildren: func(path, rootObjectPath, host, userAgent string) ([]interface{}, error) {
			if path != "/" {
				t.Errorf("path = %q", path)
			}
			return []interface{}{
				upnpav.Container{
					Object: upnpav.Object{
						ID:         "/Movies",
						ParentID:   "0",
						Restricted: 1,
						Title:      "Movies",
						Class:      "object.container.storageFolder",
					},
					ChildCount: 1,
				},
			}, nil
		},
		OnBrowseMetadata: func(string, string, string, string) (interface{}, error) {
			return upnpav.Container{}, nil
		},
		NoTranscode: true,
		NoProbe:     true,
	}
	if err := srv.Init(); err != nil {
		t.Fatal(err)
	}
	go srv.Run()
	t.Cleanup(func() { _ = srv.Close() })

	body := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:Browse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
      <ObjectID>0</ObjectID>
      <BrowseFlag>BrowseDirectChildren</BrowseFlag>
      <Filter>*</Filter>
      <StartingIndex>0</StartingIndex>
      <RequestedCount>0</RequestedCount>
      <SortCriteria></SortCriteria>
    </u:Browse>
  </s:Body>
</s:Envelope>`
	req, err := http.NewRequest(http.MethodPost, "http://"+ln.Addr().String()+"/ctl", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPACTION", `"urn:schemas-upnp-org:service:ContentDirectory:1#Browse"`)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d body %s", resp.StatusCode, raw)
	}
	if !strings.Contains(string(raw), "Movies") {
		t.Fatalf("response missing Movies: %s", raw)
	}
}

func clientAllowed(ip net.IP) bool {
	for _, network := range allowAllIPNets() {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
