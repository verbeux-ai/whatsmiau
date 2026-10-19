package whatsmiau

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var mediaBlockedNetworks = []netip.Prefix{
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

var mediaIPv6GlobalUnicast = netip.MustParsePrefix("2000::/3")

func isDisallowedMediaIP(ip net.IP) bool {
	if isDisallowedIP(ip) {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr = addr.Unmap()
	if addr.Is6() && !mediaIPv6GlobalUnicast.Contains(addr) {
		return true
	}
	for _, prefix := range mediaBlockedNetworks {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func validateMediaURL(u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.Opaque != "" {
		return fmt.Errorf("media download: an absolute HTTP(S) URL is required")
	}
	if u.User != nil {
		return fmt.Errorf("media download: URL credentials are not allowed")
	}
	host := u.Hostname()
	if strings.Contains(host, "%") {
		return fmt.Errorf("media download: scoped IP addresses are not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && isDisallowedMediaIP(ip) {
		return fmt.Errorf("media download: refusing to connect to a non-public address")
	}
	return nil
}

func mediaDialControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil || isDisallowedMediaIP(net.ParseIP(host)) {
		return fmt.Errorf("media download: refusing to connect to a non-public address")
	}
	return nil
}

type mediaHTTPTransport struct {
	transport *http.Transport
}

func (t *mediaHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := validateMediaURL(req.URL); err != nil {
		return nil, err
	}
	return t.transport.RoundTrip(req)
}

func (t *mediaHTTPTransport) CloseIdleConnections() {
	t.transport.CloseIdleConnections()
}

func newMediaHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   mediaDialControl,
	}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &mediaHTTPTransport{transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		}},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("media download: stopped after 10 redirects")
			}
			return validateMediaURL(req.URL)
		},
	}
}
