package grpcclient

import "strings"

// DialTarget returns a gRPC target that re-resolves Docker Compose service names
// after dependent containers are recreated with new IPs.
func DialTarget(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.Contains(addr, "://") {
		return addr
	}
	return "dns:///" + addr
}
