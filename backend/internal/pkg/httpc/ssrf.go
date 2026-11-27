package httpc

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

func ValidateRequestURL(ctx context.Context, rawURL string, allowPrivate bool) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported url scheme: %s", parsed.Scheme)
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return fmt.Errorf("url host is required")
	}

	if !allowPrivate {
		if err := validatePublicHost(ctx, host); err != nil {
			return err
		}
	}

	return nil
}

func validatePublicHost(ctx context.Context, host string) error {
	_, err := publicAddresses(ctx, host)
	return err
}

// Return the same checked IPs to the dialer so DNS is resolved only once.
func publicAddresses(ctx context.Context, host string) ([]net.IPAddr, error) {
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return nil, fmt.Errorf("private host is not allowed")
	}
	resolveCtx := ctx
	if resolveCtx == nil {
		resolveCtx = context.Background()
	}

	lookupCtx, cancel := context.WithTimeout(resolveCtx, 2*time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("resolve host: no addresses")
	}

	for _, addr := range addrs {
		if isPrivateOrLocalIP(addr.IP) {
			return nil, fmt.Errorf("private host is not allowed")
		}
	}

	return addrs, nil
}

func isPrivateOrLocalIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// Exclude this network, shared carrier space, and reserved IPv4 addresses.
		return v4[0] == 0 || (v4[0] == 100 && v4[1]&0xc0 == 64) || v4[0] >= 240
	}
	return false
}
