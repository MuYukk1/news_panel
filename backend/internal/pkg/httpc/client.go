package httpc

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type clientPool struct {
	mu      sync.RWMutex
	clients map[string]*http.Client
}

var defaultClientPool = &clientPool{clients: make(map[string]*http.Client)}

// NewClient creates HTTP client with specified timeout and optional proxy.
// Clients are reused by (timeout, proxy, allowPrivate) to keep connections warm.
func NewClient(timeout time.Duration, proxyURL string, allowPrivate bool) (*http.Client, error) {
	key := proxyURL + "|" + strconv.FormatInt(timeout.Milliseconds(), 10) + "|" + strconv.FormatBool(allowPrivate)

	defaultClientPool.mu.RLock()
	if client, ok := defaultClientPool.clients[key]; ok {
		defaultClientPool.mu.RUnlock()
		return client, nil
	}
	defaultClientPool.mu.RUnlock()

	transport := &http.Transport{
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          128,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if allowPrivate {
			return dialer.DialContext(ctx, network, addr)
		}
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		addresses, err := publicAddresses(ctx, host)
		if err != nil {
			return nil, err
		}
		for _, ip := range addresses {
			conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if dialErr == nil {
				return conn, nil
			}
			err = dialErr
		}
		return nil, err
	}

	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil {
			return nil, err
		}
		transport.Proxy = http.ProxyURL(proxy)
	}

	client := &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: redirectValidator(allowPrivate),
	}

	defaultClientPool.mu.Lock()
	if existing, ok := defaultClientPool.clients[key]; ok {
		defaultClientPool.mu.Unlock()
		transport.CloseIdleConnections()
		return existing, nil
	}
	defaultClientPool.clients[key] = client
	defaultClientPool.mu.Unlock()

	return client, nil
}

func redirectValidator(allowPrivate bool) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if req == nil || req.URL == nil {
			return nil
		}

		return ValidateRequestURL(req.Context(), req.URL.String(), allowPrivate)
	}
}

// SetDefaultHeaders adds default headers required for feed fetching.
func SetDefaultHeaders(req *http.Request) {
	req.Header.Set("User-Agent", "fusion/1.0")
}

// NewNoRedirectClient shares the validated transport without forwarding credentials on redirects.
func NewNoRedirectClient(timeout time.Duration, allowPrivate bool) *http.Client {
	base, _ := NewClient(timeout, "", allowPrivate)
	return &http.Client{Timeout: timeout, Transport: base.Transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
