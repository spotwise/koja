package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultSocket = "/run/koja-proxy/proxy.sock"

var (
	blockedIPv4 = mustPrefixes(
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.88.99.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
	)
	globalIPv6  = netip.MustParsePrefix("2000::/3")
	blockedIPv6 = mustPrefixes(
		"2001::/23",      // IETF protocol assignments
		"2001:db8::/32",  // documentation space
		"2002::/16",      // 6to4
		"3fff::/20",      // documentation space
		"64:ff9b::/96",   // well-known NAT64 prefix
		"64:ff9b:1::/48", // local-use NAT64 prefix
	)
)

func mustPrefixes(values ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		prefixes = append(prefixes, netip.MustParsePrefix(value))
	}
	return prefixes
}

// isPublicIP deliberately errs on the side of denying an address. IPv6 is
// limited to global-unicast space; special-purpose ranges are denied in both
// families even when the Go standard library considers them globally unicast.
func isPublicIP(address netip.Addr) bool {
	if !address.IsValid() {
		return false
	}
	address = address.WithZone("")
	if address.Is4In6() {
		address = address.Unmap()
	}
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() ||
		address.IsMulticast() || address.IsUnspecified() {
		return false
	}

	if address.Is4() {
		for _, prefix := range blockedIPv4 {
			if prefix.Contains(address) {
				return false
			}
		}
		return true
	}

	if !globalIPv6.Contains(address) {
		return false
	}
	for _, prefix := range blockedIPv6 {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}

func resolvePublic(ctx context.Context, host string) ([]netip.Addr, error) {
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("empty destination host")
	}
	if address, err := netip.ParseAddr(host); err == nil {
		if !isPublicIP(address) {
			return nil, fmt.Errorf("destination address %s is not public", address)
		}
		return []netip.Addr{address.Unmap()}, nil
	}

	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("DNS lookup failed for %q: %w", host, err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("DNS lookup returned no addresses for %q", host)
	}
	for _, address := range addresses {
		if !isPublicIP(address) {
			return nil, fmt.Errorf("DNS for %q returned non-public address %s; refusing the entire answer", host, address)
		}
	}
	return addresses, nil
}

func dialPublic(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("network %q is not supported", network)
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid destination %q: %w", address, err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, fmt.Errorf("invalid destination port %q", port)
	}

	addresses, err := resolvePublic(ctx, host)
	if err != nil {
		return nil, err
	}

	dialer := net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	var lastErr error
	for _, candidate := range addresses {
		target := net.JoinHostPort(candidate.String(), port)
		connection, dialErr := dialer.DialContext(ctx, network, target)
		if dialErr == nil {
			return connection, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = errors.New("no usable destination addresses")
	}
	return nil, lastErr
}

func destination(host, port, defaultPort string) (string, error) {
	if host == "" || strings.Contains(host, "@") {
		return "", errors.New("invalid destination authority")
	}
	if parsed, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		port = portOrDefault(port, defaultPort)
		portNumber, portErr := strconv.Atoi(port)
		if portErr != nil || portNumber < 1 || portNumber > 65535 {
			return "", fmt.Errorf("invalid destination port %q", port)
		}
		return net.JoinHostPort(parsed.String(), port), nil
	}
	if port == "" {
		// An unbracketed IPv6 literal has no explicit port.
		if strings.Count(host, ":") > 1 {
			if parsed, err := netip.ParseAddr(host); err == nil {
				return net.JoinHostPort(parsed.String(), defaultPort), nil
			}
		}
		port = defaultPort
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", fmt.Errorf("invalid destination port %q", port)
	}
	return net.JoinHostPort(host, port), nil
}

func portOrDefault(port, fallback string) string {
	if port != "" {
		return port
	}
	return fallback
}

func connectAuthority(authority string) (string, error) {
	if strings.Contains(authority, "@") {
		return "", errors.New("userinfo is not allowed in CONNECT authority")
	}
	host, port, err := net.SplitHostPort(authority)
	if err == nil {
		return destination(host, port, "443")
	}
	trimmed := strings.Trim(authority, "[]")
	if address, parseErr := netip.ParseAddr(trimmed); parseErr == nil {
		return net.JoinHostPort(address.String(), "443"), nil
	}
	if strings.Contains(authority, ":") {
		return "", errors.New("invalid CONNECT authority")
	}
	return net.JoinHostPort(authority, "443"), nil
}

type proxyHandler struct {
	transport *http.Transport
}

func newProxyHandler() http.Handler {
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialPublic,
		ForceAttemptHTTP2:     false,
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
	}
	return &proxyHandler{transport: transport}
}

func (p *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
		return
	}
	if r.URL == nil || !r.URL.IsAbs() || !strings.EqualFold(r.URL.Scheme, "http") || r.URL.Host == "" || r.URL.User != nil {
		http.Error(w, "only absolute-form HTTP requests and CONNECT are supported", http.StatusBadRequest)
		return
	}
	if r.URL.Port() != "" {
		if _, err := strconv.Atoi(r.URL.Port()); err != nil {
			http.Error(w, "invalid destination port", http.StatusBadRequest)
			return
		}
	}

	outbound := r.Clone(r.Context())
	outbound.RequestURI = ""
	outbound.Header = r.Header.Clone()
	outbound.Header.Del("Proxy-Connection")
	outbound.Header.Del("Proxy-Authorization")

	response, err := p.transport.RoundTrip(outbound)
	if err != nil {
		http.Error(w, "upstream connection refused by Koja egress policy", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	for name, values := range response.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func (p *proxyHandler) handleConnect(w http.ResponseWriter, r *http.Request) {
	target, err := connectAuthority(r.Host)
	if err != nil {
		http.Error(w, "invalid CONNECT destination", http.StatusBadRequest)
		return
	}
	upstream, err := dialPublic(r.Context(), "tcp", target)
	if err != nil {
		http.Error(w, "destination denied or unreachable by Koja egress policy", http.StatusForbidden)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "connection hijacking is unavailable", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	defer client.Close()
	defer upstream.Close()

	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}
	if bufferedBytes := buffered.Reader.Buffered(); bufferedBytes > 0 {
		if _, err := io.CopyN(upstream, buffered, int64(bufferedBytes)); err != nil {
			return
		}
	}

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, client)
		if tcp, ok := upstream.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(client, upstream)
		if tcp, ok := client.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
}

func main() {
	socketPath := os.Getenv("KOJA_PROXY_SOCKET")
	if socketPath == "" {
		socketPath = defaultSocket
	}
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("remove stale proxy socket: %v", err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("listen on proxy socket: %v", err)
	}
	defer listener.Close()
	if err := os.Chmod(socketPath, 0600); err != nil {
		log.Fatalf("set proxy socket permissions: %v", err)
	}

	server := &http.Server{
		Handler:           newProxyHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("Koja egress proxy listening on Unix socket %s", socketPath)
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("proxy server: %v", err)
	}
}
