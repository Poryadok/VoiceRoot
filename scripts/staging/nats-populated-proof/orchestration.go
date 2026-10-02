package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type dockerExecutor interface {
	run(context.Context, ...string) ([]byte, error)
}
type realDocker struct{}

func (realDocker) run(ctx context.Context, args ...string) ([]byte, error) {
	raw, e := exec.CommandContext(ctx, "docker", args...).Output()
	if e != nil {
		var failed *exec.ExitError
		if len(args) > 0 && args[0] == "exec" && errors.As(e, &failed) {
			if code := safeRuntimeCode(string(failed.Stderr)); code != "" {
				return nil, failure("runtime_phase_rejected:" + code)
			}
		}
		return nil, failure("docker_command_failed")
	}
	return raw, nil
}
func safeRuntimeCode(raw string) string {
	allowed := strings.Fields("runtime_container_context_missing runtime_context_invalid runtime_owned_identity_not_proven runtime_kind_invalid custody_trust_anchor_not_enrolled runtime_context_stale runtime_network_identity_not_proven runtime_external_route_present runtime_privilege_boundary_not_proven runtime_duplicate_mount runtime_owned_mount_not_proven runtime_container_id_not_proven runtime_unowned_mount_present runtime_mount_set_not_proven runtime_file_set_not_proven runtime_owned_bytes_not_proven runtime_mount_identity_invalid clone_connect_failed clone_request_failed")
	for _, line := range strings.Split(raw, "\n") {
		code := strings.TrimPrefix(strings.TrimSpace(line), "NATS_POPULATED_ACL_PROOF=FAIL code=")
		for _, known := range allowed {
			if code == known {
				return known
			}
		}
	}
	return ""
}

type sandbox struct {
	docker                                                         dockerExecutor
	owner, network, networkID, subnet, volume, work, helper, hubIP string
	containers                                                     []string
	input                                                          *verifiedInput
}

func newSandbox(d dockerExecutor, helper string) (*sandbox, error) {
	id := make([]byte, 12)
	if _, e := rand.Read(id); e != nil {
		return nil, failure("sandbox_identity_failed")
	}
	owner := "voice-populated-" + hex.EncodeToString(id)
	return &sandbox{docker: d, owner: owner, network: owner, volume: owner + "-data", helper: helper}, nil
}
func (s *sandbox) command(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return s.docker.run(ctx, args...)
}
func (s *sandbox) cleanup() error {
	failed := false
	for _, name := range s.containers {
		if _, e := s.command("rm", "-f", name); e != nil {
			failed = true
		}
	}
	if _, e := s.command("volume", "rm", s.volume); e != nil {
		failed = true
	}
	if _, e := s.command("network", "rm", s.network); e != nil {
		failed = true
	}
	for _, kind := range []string{"container", "volume", "network"} {
		format := "{{.ID}}"
		args := []string{kind, "ls"}
		if kind == "container" {
			args = append(args, "--all")
		}
		if kind == "volume" {
			format = "{{.Name}}"
		}
		raw, e := s.command(append(args, "--filter", "label=voice.populated-proof.owner="+s.owner, "--format", format)...)
		if e != nil || strings.TrimSpace(string(raw)) != "" {
			failed = true
		}
	}
	if s.work != "" {
		if os.RemoveAll(s.work) != nil {
			failed = true
		}
	}
	if failed {
		return failure("sandbox_cleanup_failed")
	}
	return nil
}
func writeOpaque(path string, raw []byte) error {
	if os.WriteFile(path, raw, 0600) != nil {
		return failure("owned_input_write_failed")
	}
	return nil
}
func (s *sandbox) prepare(in *verifiedInput) error {
	s.input = in
	work, e := os.MkdirTemp("", s.owner+"-owned-")
	if e != nil {
		return failure("owned_directory_failed")
	}
	s.work = work
	for _, dir := range []string{"jwt", "tls", "creds"} {
		if os.Mkdir(filepath.Join(work, dir), 0700) != nil {
			return failure("owned_directory_failed")
		}
	}
	for name, raw := range in.Files {
		dir := work
		switch {
		case strings.HasSuffix(name, ".jwt") || strings.HasSuffix(name, ".public"):
			dir = filepath.Join(work, "jwt")
		case name == "tls.crt" || name == "tls.key" || name == "ca.crt":
			dir = filepath.Join(work, "tls")
		case strings.HasSuffix(name, ".creds"):
			dir = filepath.Join(work, "creds")
		}
		if e = writeOpaque(filepath.Join(dir, name), raw); e != nil {
			return e
		}
	}
	if _, e = s.command("network", "create", "--internal", "--opt", "com.docker.network.bridge.inhibit_ipv4=true", "--label", "voice.populated-proof.owner="+s.owner, s.network); e != nil {
		return e
	}
	raw, e := s.command("network", "inspect", s.network, "--format", "{{json .}}")
	if e != nil {
		return e
	}
	var n struct {
		Id                   string
		Internal, EnableIPv6 bool
		Options, Labels      map[string]string
		IPAM                 struct{ Config []struct{ Subnet string } }
	}
	if json.Unmarshal(raw, &n) != nil || !n.Internal || n.EnableIPv6 || n.Options["com.docker.network.bridge.inhibit_ipv4"] != "true" || n.Labels["voice.populated-proof.owner"] != s.owner || !digestPattern.MatchString(n.Id) || len(n.IPAM.Config) != 1 {
		return failure("network_isolation_not_proven")
	}
	s.networkID = n.Id
	s.subnet = n.IPAM.Config[0].Subnet
	if _, e = s.command("volume", "create", "--label", "voice.populated-proof.owner="+s.owner, s.volume); e != nil {
		return e
	}
	return s.verifyVolume("")
}

type inspectedMount struct {
	Type, Name, Source, Destination string
	RW                              bool
}
type inspectedContainer struct {
	Id, Image  string
	HostConfig struct {
		Privileged, ReadonlyRootfs bool
		NetworkMode                string
		CapDrop, SecurityOpt       []string
		PortBindings               map[string]any
	}
	NetworkSettings struct {
		Networks map[string]struct{ IPAddress, NetworkID string }
	}
	Mounts []inspectedMount
	Config struct {
		Image, Hostname string
		Env             []string
	}
}

func (s *sandbox) verifyVolume(actualSource string) error {
	raw, e := s.command("volume", "inspect", s.volume, "--format", "{{json .}}")
	if e != nil {
		return failure("owned_volume_not_proven")
	}
	var v struct {
		Name, Driver, Scope, Mountpoint string
		Labels, Options                 map[string]string
	}
	if json.Unmarshal(raw, &v) != nil || v.Name != s.volume || v.Driver != "local" || v.Scope != "local" || v.Labels["voice.populated-proof.owner"] != s.owner || len(v.Options) != 0 || v.Mountpoint == "" || actualSource != "" && v.Mountpoint != actualSource {
		return failure("owned_volume_not_proven")
	}
	return nil
}
func (s *sandbox) expectedMounts(role string) map[string]string {
	if role == "hub" {
		return map[string]string{"/etc/nats/hub.conf": filepath.Join(s.work, "hub.conf"), "/etc/nats/jwt/operator.jwt": filepath.Join(s.work, "jwt", "operator.jwt"), "/etc/nats/tls/tls.crt": filepath.Join(s.work, "tls", "tls.crt"), "/etc/nats/tls/tls.key": filepath.Join(s.work, "tls", "tls.key"), "/etc/nats/tls/ca.crt": filepath.Join(s.work, "tls", "ca.crt"), "/data": ""}
	}
	m := map[string]string{"/proof": s.helper, contextPath: filepath.Join(s.work, "context-"+role+".json")}
	if role == "diagnostic" {
		for _, profile := range []string{"bootstrap", "social", "realtime"} {
			m["/var/run/nats/proof-creds/"+profile+".creds"] = filepath.Join(s.work, "creds", profile+".creds")
		}
	} else {
		m["/etc/nats/leaf.conf"] = filepath.Join(s.work, "leaf.conf")
		m["/etc/nats/tls/ca.crt"] = filepath.Join(s.work, "tls", "ca.crt")
		m["/var/run/nats/creds/"+role+".creds"] = filepath.Join(s.work, "creds", role+".creds")
	}
	return m
}
func (s *sandbox) inspectHolder(name, role string, started bool) (*inspectedContainer, error) {
	raw, e := s.command("inspect", name, "--format", "{{json .}}")
	if e != nil {
		return nil, failure("container_isolation_not_proven")
	}
	var c inspectedContainer
	if json.Unmarshal(raw, &c) != nil || !digestPattern.MatchString(c.Id) || c.Config.Hostname != name || c.HostConfig.Privileged || !c.HostConfig.ReadonlyRootfs || c.HostConfig.NetworkMode != s.network || len(c.HostConfig.PortBindings) != 0 || len(c.NetworkSettings.Networks) != 1 || !sameSet(c.HostConfig.CapDrop, []string{"ALL"}) || !sameSet(c.HostConfig.SecurityOpt, []string{"no-new-privileges"}) || c.Config.Image != natsImage {
		return nil, failure("container_isolation_not_proven")
	}
	network, ok := c.NetworkSettings.Networks[s.network]
	if !ok || started && (network.NetworkID != s.networkID || network.IPAddress == "") {
		return nil, failure("container_network_not_proven")
	}
	expected := s.expectedMounts(role)
	if len(c.Mounts) != len(expected) {
		return nil, failure("container_mount_count_not_proven")
	}
	seen := map[string]bool{}
	for _, m := range c.Mounts {
		source, ok := expected[m.Destination]
		if !ok || seen[m.Destination] {
			return nil, failure("container_mounts_not_proven")
		}
		seen[m.Destination] = true
		if m.Destination == "/data" {
			if role != "hub" || m.Type != "volume" || !m.RW || m.Name != s.volume {
				return nil, failure("owned_volume_not_proven")
			}
			if e = s.verifyVolume(m.Source); e != nil {
				return nil, e
			}
		} else if m.Type != "bind" || m.RW {
			return nil, failure("container_mount_type_readonly_not_proven")
		} else if !sameHostMountSource(m.Source, source) {
			return nil, failure("container_mount_source_not_proven")
		}
	}
	image, e := s.command("image", "inspect", natsImage, "--format", "{{.Id}}")
	if e != nil || strings.TrimSpace(string(image)) != c.Image {
		return nil, failure("container_image_not_proven")
	}
	return &c, nil
}

func sameHostMountSource(actual, expected string) bool {
	if filepath.Clean(actual) == filepath.Clean(expected) {
		return true
	}
	// Docker Desktop reports individual Windows file binds in VM coordinates.
	// Only disposable fixtures execute on Windows; production custody is Linux.
	if runtime.GOOS == "windows" && len(expected) > 2 && expected[1] == ':' {
		vm := "/run/desktop/mnt/host/" + strings.ToLower(expected[:1]) + strings.ReplaceAll(expected[2:], `\`, "/")
		return actual == vm
	}
	return false
}
func (s *sandbox) launch(name, alias, config, credential string) error {
	role := credential
	if role == "" {
		role = "hub"
	}
	mounts := s.expectedMounts(role)
	if role != "hub" {
		if e := writeOpaque(mounts[contextPath], []byte("{}")); e != nil {
			return e
		}
	}
	args := []string{"create", "--name", name, "--hostname", name, "--network", s.network, "--network-alias", alias, "--label", "voice.populated-proof.owner=" + s.owner, "--read-only", "--cap-drop=ALL", "--security-opt=no-new-privileges", "--tmpfs", "/tmp:rw,noexec,nosuid,size=16m"}
	for dest, src := range mounts {
		if dest == "/data" {
			args = append(args, "--mount", "type=volume,src="+s.volume+",dst=/data")
		} else {
			args = append(args, "--mount", "type=bind,src="+src+",dst="+dest+",readonly")
		}
	}
	if role == "diagnostic" {
		args = append(args, "--entrypoint", "/bin/sleep", natsImage, "600")
	} else {
		if role != "hub" {
			args = append(args, "--env", "NATS_CREDS=/var/run/nats/creds/"+role+".creds")
		}
		args = append(args, natsImage, "--config", "/etc/nats/"+config)
	}
	s.containers = append(s.containers, name)
	if _, e := s.command(args...); e != nil {
		return e
	}
	if _, e := s.inspectHolder(name, role, false); e != nil {
		return e
	}
	if _, e := s.command("start", name); e != nil {
		return e
	}
	actual, e := s.inspectHolder(name, role, true)
	if e != nil {
		return e
	}
	address := actual.NetworkSettings.Networks[s.network].IPAddress
	if role == "hub" {
		s.hubIP = address
		return nil
	}
	c := runtimeContext{Schema: schema, Kind: s.input.Manifest.Kind, Owner: s.owner, Container: name, ContainerID: actual.Id, Role: role, IP: address, Subnet: s.subnet, HubIP: s.hubIP, CreatedAt: time.Now().UTC(), ManifestSHA: s.input.ManifestSHA, FileHashes: map[string]string{}}
	for dest, src := range mounts {
		if dest == contextPath {
			continue
		}
		raw, e := os.ReadFile(src)
		if e != nil {
			return failure("owned_runtime_input_failed")
		}
		c.FileHashes[dest] = hash(raw)
	}
	raw, e := json.Marshal(c)
	if e != nil {
		return failure("runtime_context_encode_failed")
	}
	return writeOpaque(mounts[contextPath], raw)
}
func (s *sandbox) phase(container, phase string) error {
	_, e := s.command("exec", container, "/proof", "runtime", phase)
	return e
}
func (s *sandbox) waitReady(container string) error {
	deadline := time.Now().Add(15 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		if last = s.phase(container, "ready"); last == nil {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	if last != nil {
		return failure("clone_readiness_failed:" + last.Error())
	}
	return failure("clone_readiness_failed")
}
func runSandbox(in *verifiedInput, d dockerExecutor, helper string) (err error) {
	if runtime.GOOS != "linux" && in.Manifest.Kind != "fixture" {
		return failure("sandbox_requires_linux_custody_host")
	}
	helperAbs, e := filepath.Abs(helper)
	if e != nil {
		return failure("helper_invalid")
	}
	info, e := os.Lstat(helperAbs)
	if e != nil || !info.Mode().IsRegular() || runtime.GOOS == "linux" && info.Mode().Perm()&0111 == 0 {
		return failure("helper_invalid")
	}
	s, e := newSandbox(d, helperAbs)
	if e != nil {
		return e
	}
	defer func() {
		if e := s.cleanup(); e != nil {
			if err == nil {
				err = e
			} else {
				err = failure(e.Error() + ":primary:" + err.Error())
			}
		}
	}()
	if e = s.prepare(in); e != nil {
		return e
	}
	hub, diagnostic := s.owner+"-hub", s.owner+"-diagnostic"
	if e = s.launch(hub, "voice-nats", "hub.conf", ""); e != nil {
		return e
	}
	if e = s.launch(diagnostic, "proof-diagnostic", "", "diagnostic"); e != nil {
		return e
	}
	if e = s.waitReady(diagnostic); e != nil {
		return e
	}
	if e = s.phase(diagnostic, "setup"); e != nil {
		return failure("clone_setup_failed")
	}
	if e = s.phase(diagnostic, "exercise"); e != nil {
		return failure("clone_authorization_failed")
	}
	if _, e = s.command("restart", hub); e != nil {
		return e
	}
	if e = s.waitReady(diagnostic); e != nil {
		return e
	}
	if e = s.phase(diagnostic, "persisted"); e != nil {
		return failure("clone_persistence_failed")
	}
	realtime, social := s.owner+"-realtime", s.owner+"-social"
	if e = s.launch(realtime, "proof-realtime", "leaf.conf", "realtime"); e != nil {
		return e
	}
	if e = s.launch(social, "proof-social", "leaf.conf", "social"); e != nil {
		return e
	}
	received := make(chan error, 1)
	go func() { received <- s.phase(realtime, "leaf-receive") }()
	deadline := time.Now().Add(10 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		if s.phase(realtime, "leaf-ready") == nil {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		return failure("leaf_subscription_readiness_failed")
	}
	if e = s.phase(social, "leaf-publish"); e != nil {
		return failure("leaf_publishing_failed")
	}
	if e = <-received; e != nil {
		return failure("leaf_delivery_ack_failed")
	}
	if _, e = s.command("restart", hub); e != nil {
		return e
	}
	if e = s.waitReady(diagnostic); e != nil {
		return e
	}
	return s.phase(diagnostic, "leaf-persisted")
}
