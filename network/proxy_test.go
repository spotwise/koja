package main

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		address string
		want    bool
	}{
		{"1.1.1.1", true},
		{"8.8.8.8", true},
		{"2606:4700:4700::1111", true},
		{"2001:4860:4860::8888", true},
		{"10.0.0.1", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},
		{"127.0.0.1", false},
		{"169.254.169.254", false},
		{"100.64.0.1", false},
		{"192.0.2.1", false},
		{"198.18.0.1", false},
		{"224.0.0.1", false},
		{"255.255.255.255", false},
		{"::", false},
		{"::1", false},
		{"fc00::1", false},
		{"fe80::1", false},
		{"ff02::1", false},
		{"2001:db8::1", false},
		{"2002::1", false},
		{"3fff::1", false},
		{"::ffff:192.168.1.1", false},
		{"64:ff9b::a00:1", false},
	}

	for _, test := range tests {
		t.Run(test.address, func(t *testing.T) {
			address := netip.MustParseAddr(test.address)
			if got := isPublicIP(address); got != test.want {
				t.Fatalf("isPublicIP(%s) = %t, want %t", address, got, test.want)
			}
		})
	}
}

func TestConnectAuthority(t *testing.T) {
	tests := []struct {
		input string
		want  string
		fail  bool
	}{
		{"example.com:443", "example.com:443", false},
		{"example.com", "example.com:443", false},
		{"[2606:4700:4700::1111]:443", "[2606:4700:4700::1111]:443", false},
		{"127.0.0.1:443", "127.0.0.1:443", false}, // Address policy is enforced by dialPublic.
		{"user@example.com:443", "", true},
		{"example.com:bad", "", true},
		{"[2606:4700:4700::1111]:bad", "", true},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := connectAuthority(test.input)
			if (err != nil) != test.fail {
				t.Fatalf("connectAuthority error = %v, want failure %t", err, test.fail)
			}
			if !test.fail && got != test.want {
				t.Fatalf("connectAuthority = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolvePublicIPLiteral(t *testing.T) {
	if _, err := resolvePublic(context.Background(), "127.0.0.1"); err == nil {
		t.Fatal("expected loopback destination to be rejected")
	}
	addresses, err := resolvePublic(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatalf("public IP literal was rejected: %v", err)
	}
	if len(addresses) != 1 || addresses[0] != netip.MustParseAddr("1.1.1.1") {
		t.Fatalf("unexpected resolved addresses: %v", addresses)
	}
}

func TestProxyRejectsPrivateDestinations(t *testing.T) {
	listener, err := net.Listen("unix", t.TempDir()+"/proxy.sock")
	if err != nil {
		t.Fatalf("listen on test socket: %v", err)
	}
	server := &http.Server{Handler: newProxyHandler()}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})

	proxyURL, err := url.Parse("http://koja-proxy.invalid:3128")
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL),
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", listener.Addr().String())
		},
	}
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	t.Cleanup(transport.CloseIdleConnections)

	response, err := client.Get("http://127.0.0.1/")
	if err != nil {
		t.Fatalf("HTTP request did not reach proxy: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("private HTTP destination returned status %d, want %d", response.StatusCode, http.StatusBadGateway)
	}

	_, err = client.Get("https://127.0.0.1/")
	if err == nil || !strings.Contains(err.Error(), "Forbidden") {
		t.Fatalf("private HTTPS CONNECT should be rejected with 403 Forbidden, got %v", err)
	}
}
