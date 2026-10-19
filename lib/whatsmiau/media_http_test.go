package whatsmiau

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMediaDialControl(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "10.0.0.1", "172.16.0.1", "192.168.1.1",
		"169.254.169.254", "169.254.170.2", "100.100.100.200", "0.0.0.0", "0.1.2.3",
		"192.0.0.1", "198.18.0.1", "192.0.2.1", "198.51.100.1", "203.0.113.1",
		"224.0.0.1", "240.0.0.1", "255.255.255.255",
		"::", "::1", "fe80::1", "fc00::1", "fd00:ec2::254", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:169.254.169.254", "::127.0.0.1",
		"64:ff9b::a9fe:a9fe", "64:ff9b:1::a00:1", "100::1",
		"2001::1", "2001:db8::1", "2002:7f00:1::", "3fff::1",
	}
	for _, host := range blocked {
		t.Run(host, func(t *testing.T) {
			if err := mediaDialControl("tcp", net.JoinHostPort(host, "80"), nil); err == nil {
				t.Fatal("accepted a non-public destination")
			}
		})
	}
	for _, host := range []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "::ffff:8.8.8.8", "2606:4700::1111", "2001:4860:4860::8888"} {
		if err := mediaDialControl("tcp", net.JoinHostPort(host, "443"), nil); err != nil {
			t.Errorf("public IP %s rejected: %v", host, err)
		}
	}
	for _, address := range []string{"invalid", "example.com:443", "[fe80::1%eth0]:80"} {
		if err := mediaDialControl("tcp", address, nil); err == nil {
			t.Errorf("invalid dial address %s accepted", address)
		}
	}
}

func TestMediaHTTPClientRejectsUnsafeURLs(t *testing.T) {
	client := newMediaHTTPClient()
	defer client.CloseIdleConnections()
	for _, target := range []string{
		"http://127.0.0.1/media", "https://[::1]/media", "http://10.0.0.1/media",
		"http://169.254.169.254/latest/meta-data/", "http://[::ffff:127.0.0.1]/media",
		"http://[fe80::1%25eth0]/media", "http://user:password@example.com/media",
		"file:///etc/passwd", "ftp://example.com/media", "/media", "http:///media",
	} {
		t.Run(target, func(t *testing.T) {
			resp, err := client.Get(target)
			if resp != nil {
				resp.Body.Close()
			}
			if err == nil {
				t.Fatal("unsafe media URL was accepted")
			}
		})
	}
	for _, target := range []string{"http://example.com/media", "https://example.com/media?signature=a%2Fb", "https://[2606:4700::1111]/media"} {
		u, err := url.Parse(target)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateMediaURL(u); err != nil {
			t.Errorf("public URL rejected: %v", err)
		}
	}
}

func TestMediaHTTPClientBlocksResolvedLoopback(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte("private"))
	}))
	defer srv.Close()
	_, port, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	client := newMediaHTTPClient()
	defer client.CloseIdleConnections()
	for _, scheme := range []string{"http", "https"} {
		// The URL has no IP literal: the production dialer must reject the
		// actual address after resolving DNS, before any HTTP/TLS bytes flow.
		resp, err := client.Get(scheme + "://localhost:" + port + "/media")
		if resp != nil {
			resp.Body.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "non-public address") {
			t.Fatalf("expected rejection after DNS resolution, got %v", err)
		}
	}
	if hits.Load() != 0 {
		t.Fatal("internal service received a media request")
	}
}

func TestMediaDownloadRedirects(t *testing.T) {
	var privateHits atomic.Int32
	private := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		privateHits.Add(1)
	}))
	defer private.Close()
	privateURL, _ := url.Parse(private.URL)
	localhostURL := "http://localhost:" + privateURL.Port() + "/secret"
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/public":
			http.Redirect(w, r, "http://cdn.example.com/media?signature=a%2Fb", http.StatusFound)
		case "/private":
			http.Redirect(w, r, private.URL, http.StatusFound)
		case "/dns-private":
			http.Redirect(w, r, localhostURL, http.StatusFound)
		case "/metadata":
			http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
		case "/credentials":
			http.Redirect(w, r, "http://user:secret@cdn.example.com/media", http.StatusFound)
		case "/scheme":
			http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/media":
			if r.URL.RawQuery != "signature=a%2Fb" {
				t.Error("signed media URL query changed")
			}
			io.WriteString(w, "media bytes")
		default:
			http.NotFound(w, r)
		}
	}))
	defer public.Close()

	client := newMediaHTTPClient()
	defer client.CloseIdleConnections()
	transport := client.Transport.(*mediaHTTPTransport).transport
	productionDial := transport.DialContext
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		// Only the simulated public origins are routed to the fixture. Every
		// other destination, including redirect hostnames, uses the real guard.
		if address == "media.example.com:80" || address == "cdn.example.com:80" {
			return (&net.Dialer{}).DialContext(ctx, network, public.Listener.Addr().String())
		}
		return productionDial(ctx, network, address)
	}
	s := &Whatsmiau{httpClient: client}
	data, err := s.fetchBytes(context.Background(), "http://media.example.com/public")
	if err != nil || string(data) != "media bytes" {
		t.Fatalf("public redirect download: data=%q, err=%v", data, err)
	}
	for _, path := range []string{"private", "dns-private", "metadata", "credentials", "scheme", "loop"} {
		t.Run(path, func(t *testing.T) {
			if _, err := s.fetchBytes(context.Background(), "http://media.example.com/"+path); err == nil {
				t.Fatal("unsafe redirect or redirect loop was accepted")
			}
		})
	}
	if privateHits.Load() != 0 {
		t.Fatal("redirect reached the internal service")
	}
}

func TestMediaHTTPClientDoesNotUseEnvironmentProxy(t *testing.T) {
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"} {
		t.Setenv(key, "http://proxy.example.com:3128")
	}
	client := newMediaHTTPClient()
	defer client.CloseIdleConnections()
	if client.Transport.(*mediaHTTPTransport).transport.Proxy != nil {
		t.Fatal("environment proxy could bypass destination IP validation")
	}
}

func TestMediaDownloadPreservesBase64(t *testing.T) {
	s := &Whatsmiau{httpClient: newMediaHTTPClient()}
	defer s.httpClient.CloseIdleConnections()
	data, err := s.fetchBytes(context.Background(), "data:application/octet-stream;base64,bWVkaWE=")
	if err != nil || string(data) != "media" {
		t.Fatalf("base64 media changed: data=%q, err=%v", data, err)
	}
}
