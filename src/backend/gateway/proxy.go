package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

func proxyFromEnv(name string, logger *slog.Logger) http.Handler {
	return reverseProxy(os.Getenv(name), logger)
}

func reverseProxy(rawURL string, logger *slog.Logger) http.Handler {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	target, err := url.Parse(rawURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		if logger != nil {
			logger.Warn("invalid upstream URL", slog.String("url", rawURL), slog.Any("error", err))
		}
		return nil
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	}
	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalDirector(r)
		r.Host = target.Host
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		status := http.StatusBadGateway
		if errors.Is(err, context.Canceled) {
			status = http.StatusServiceUnavailable
		}
		writeJSON(w, status, map[string]string{"error": "upstream_unavailable"})
	}
	return proxy
}
