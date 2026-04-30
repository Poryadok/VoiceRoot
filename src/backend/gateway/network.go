package main

import (
	"net"
	"net/http"
	"strings"
)

type trustedProxy struct {
	network *net.IPNet
}

func parseTrustedProxies(cidrs []string) []trustedProxy {
	proxies := make([]trustedProxy, 0, len(cidrs))
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			ip := net.ParseIP(raw)
			if ip == nil {
				continue
			}
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			network = &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)}
		}
		proxies = append(proxies, trustedProxy{network: network})
	}
	return proxies
}

func (g *gateway) clientIP(r *http.Request) string {
	remote := remoteIP(r.RemoteAddr)
	if g.isTrustedProxy(remote) {
		if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
			ip, _, _ := strings.Cut(forwardedFor, ",")
			return strings.TrimSpace(ip)
		}
	}
	return remote
}

func (g *gateway) isTrustedProxy(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, proxy := range g.trustedProxies {
		if proxy.network.Contains(parsed) {
			return true
		}
	}
	return false
}

func remoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}
