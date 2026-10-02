package main

import (
	"net"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const contextPath = "/run/voice-populated/context.json"

var ownerPattern = regexp.MustCompile(`^voice-populated-[a-f0-9]{24}$`)

type runtimeContext struct {
	Schema      string            `json:"schema"`
	Kind        string            `json:"kind"`
	Owner       string            `json:"owner"`
	Container   string            `json:"container"`
	ContainerID string            `json:"container_id"`
	Role        string            `json:"role"`
	IP          string            `json:"ip"`
	Subnet      string            `json:"subnet"`
	HubIP       string            `json:"hub_ip"`
	CreatedAt   time.Time         `json:"created_at"`
	ManifestSHA string            `json:"manifest_sha"`
	FileHashes  map[string]string `json:"file_hashes"`
}
type runtimeMount struct {
	root, path, fs string
	readonly       bool
}
type runtimeIdentity struct {
	hostname  string
	mounts    []runtimeMount
	addresses []string
	routes    string
	status    string
}

func allowedPhase(role, phase string) bool {
	switch role {
	case "diagnostic":
		return strings.Contains(" ready setup exercise persisted leaf-persisted ", " "+phase+" ")
	case "realtime":
		return phase == "leaf-receive" || phase == "leaf-ready"
	case "social":
		return phase == "leaf-publish"
	}
	return false
}
func contextFiles(role string) []string {
	if role == "diagnostic" {
		return []string{"/proof", contextPath, "/var/run/nats/proof-creds/bootstrap.creds", "/var/run/nats/proof-creds/social.creds", "/var/run/nats/proof-creds/realtime.creds"}
	}
	return []string{"/proof", contextPath, "/etc/nats/leaf.conf", "/etc/nats/tls/ca.crt", "/var/run/nats/creds/" + role + ".creds"}
}

// The pinned Alpine image resolves /var/run to /run. No arbitrary path alias,
// added bind or alternate root is accepted by the actual mount-set guard.
func runtimeMountPath(path string) string {
	if strings.HasPrefix(path, "/var/run/") {
		return strings.TrimPrefix(path, "/var")
	}
	return path
}
func parseMounts(raw string) ([]runtimeMount, error) {
	var out []runtimeMount
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		parts := strings.Split(line, " - ")
		if len(parts) != 2 {
			return nil, failure("runtime_mount_identity_invalid")
		}
		left, right := strings.Fields(parts[0]), strings.Fields(parts[1])
		if len(left) < 6 || len(right) < 3 {
			return nil, failure("runtime_mount_identity_invalid")
		}
		out = append(out, runtimeMount{root: unescapeMount(left[3]), path: unescapeMount(left[4]), fs: right[0], readonly: strings.Contains(","+left[5]+",", ",ro,")})
	}
	return out, nil
}
func unescapeMount(s string) string {
	return strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(s)
}
func verifyRuntimeContext(c runtimeContext, id runtimeIdentity, phase string, now time.Time) error {
	if c.Schema != schema || !ownerPattern.MatchString(c.Owner) || c.Container != c.Owner+"-"+c.Role || !digestPattern.MatchString(c.ContainerID) || !digestPattern.MatchString(c.ManifestSHA) || id.hostname != c.Container || !allowedPhase(c.Role, phase) {
		return failure("runtime_owned_identity_not_proven")
	}
	if c.Kind != "fixture" && c.Kind != "candidate" && c.Kind != "deployed" {
		return failure("runtime_kind_invalid")
	}
	if c.Kind != "fixture" && !digestPattern.MatchString(approvedTrustPolicySHA) {
		return failure("custody_trust_anchor_not_enrolled")
	}
	if now.Sub(c.CreatedAt) > 5*time.Minute || c.CreatedAt.After(now.Add(30*time.Second)) {
		return failure("runtime_context_stale")
	}
	ip, hub := net.ParseIP(c.IP), net.ParseIP(c.HubIP)
	_, subnet, e := net.ParseCIDR(c.Subnet)
	if e != nil || ip == nil || ip.To4() == nil || hub == nil || hub.To4() == nil || !subnet.Contains(ip) || !subnet.Contains(hub) || len(id.addresses) != 1 || id.addresses[0] != c.IP {
		return failure("runtime_network_identity_not_proven")
	}
	for _, line := range strings.Split(id.routes, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] == "Iface" {
			continue
		}
		gateway, e := strconv.ParseUint(fields[2], 16, 32)
		if e != nil || gateway != 0 || fields[1] == "00000000" || fields[0] != "eth0" {
			return failure("runtime_external_route_present")
		}
	}
	if !strings.Contains(id.status, "NoNewPrivs:\t1") || !strings.Contains(id.status, "CapEff:\t0000000000000000") {
		return failure("runtime_privilege_boundary_not_proven")
	}
	expected := map[string]bool{}
	for _, path := range contextFiles(c.Role) {
		expected[runtimeMountPath(path)] = true
	}
	seen := map[string]bool{}
	rootOK, hostnameOK := false, false
	for _, m := range id.mounts {
		m.path = runtimeMountPath(m.path)
		if seen[m.path] {
			return failure("runtime_duplicate_mount")
		}
		seen[m.path] = true
		if expected[m.path] {
			if !m.readonly {
				return failure("runtime_owned_mount_not_proven")
			}
			if m.path != "/proof" && !strings.Contains(m.root, "/"+c.Owner+"-owned-") {
				return failure("runtime_owned_mount_not_proven")
			}
			delete(expected, m.path)
			continue
		}
		if m.path == "/" {
			rootOK = m.readonly && m.fs == "overlay"
			continue
		}
		if m.path == "/etc/hostname" || m.path == "/etc/hosts" || m.path == "/etc/resolv.conf" {
			if !strings.Contains(m.root, "/"+c.ContainerID+"/") {
				return failure("runtime_container_id_not_proven")
			}
			if m.path == "/etc/hostname" {
				hostnameOK = true
			}
			continue
		}
		if m.path == "/tmp" && m.fs == "tmpfs" {
			continue
		}
		if (m.path == "/proc" || strings.HasPrefix(m.path, "/proc/")) && (m.fs == "proc" || m.fs == "tmpfs") {
			continue
		}
		if strings.HasPrefix(m.path, "/sys/fs/cgroup/") && m.fs == "cgroup" && m.readonly {
			continue
		}
		if (m.path == "/sys" || strings.HasPrefix(m.path, "/sys/")) && (m.fs == "sysfs" || m.fs == "cgroup2" && m.readonly || m.fs == "tmpfs") {
			continue
		}
		if (m.path == "/dev" || strings.HasPrefix(m.path, "/dev/")) && (m.fs == "tmpfs" || m.fs == "devpts" || m.fs == "mqueue") {
			continue
		}
		return failure("runtime_unowned_mount_present")
	}
	if len(expected) != 0 || !rootOK || !hostnameOK {
		return failure("runtime_mount_set_not_proven")
	}
	return nil
}
func loadRuntimeContext(phase string) (*runtimeContext, error) {
	// Reject ambient host use before accepting arbitrary paths, DNS or creds.
	// The fixed context is the only file read before its namespace/mount guard.
	if runtime.GOOS != "linux" {
		return nil, failure("runtime_container_context_missing")
	}
	info, e := os.Lstat(contextPath)
	if e != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 8192 {
		return nil, failure("runtime_container_context_missing")
	}
	// This context contains only public coordinates/hashes. Authority is not its
	// file mode: the actual namespace/readonly mounts and parent contract are
	// verified below before any protected credential read or connection.
	raw, e := os.ReadFile(contextPath)
	if e != nil {
		return nil, failure("runtime_container_context_missing")
	}
	var c runtimeContext
	if decodeStrict(raw, &c) != nil {
		return nil, failure("runtime_context_invalid")
	}
	hostname, e := os.Hostname()
	if e != nil {
		return nil, failure("runtime_container_context_missing")
	}
	mountBytes, e := os.ReadFile("/proc/self/mountinfo")
	if e != nil {
		return nil, failure("runtime_container_context_missing")
	}
	mounts, e := parseMounts(string(mountBytes))
	if e != nil {
		return nil, e
	}
	routes, e := os.ReadFile("/proc/net/route")
	if e != nil {
		return nil, failure("runtime_container_context_missing")
	}
	status, e := os.ReadFile("/proc/self/status")
	if e != nil {
		return nil, failure("runtime_container_context_missing")
	}
	interfaces, e := net.Interfaces()
	if e != nil {
		return nil, failure("runtime_container_context_missing")
	}
	var addresses []string
	for _, i := range interfaces {
		if i.Name == "lo" {
			continue
		}
		if i.Name != "eth0" {
			return nil, failure("runtime_network_identity_not_proven")
		}
		values, e := i.Addrs()
		if e != nil {
			return nil, failure("runtime_network_identity_not_proven")
		}
		for _, v := range values {
			ip, _, e := net.ParseCIDR(v.String())
			if e != nil || ip.To4() == nil {
				return nil, failure("runtime_network_identity_not_proven")
			}
			addresses = append(addresses, ip.String())
		}
	}
	if e = verifyRuntimeContext(c, runtimeIdentity{hostname: hostname, mounts: mounts, addresses: addresses, routes: string(routes), status: string(status)}, phase, time.Now()); e != nil {
		return nil, e
	}
	files := contextFiles(c.Role)
	if len(c.FileHashes) != len(files)-1 {
		return nil, failure("runtime_file_set_not_proven")
	}
	for _, path := range files {
		if path == contextPath {
			continue
		}
		raw, e := os.ReadFile(path)
		if e != nil || !digestPattern.MatchString(c.FileHashes[path]) || hash(raw) != c.FileHashes[path] {
			return nil, failure("runtime_owned_bytes_not_proven")
		}
	}
	return &c, nil
}
