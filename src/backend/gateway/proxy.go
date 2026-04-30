package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

func proxyFromEnv(name string) http.Handler {
	return reverseProxy(os.Getenv(name))
}

func reverseProxy(rawURL string) http.Handler {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	target, err := url.Parse(rawURL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		log.Printf("invalid upstream URL %q: %v", rawURL, err)
		return nil
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
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
