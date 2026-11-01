// ci-rootless-create creates only the narrowly specified local CI daemon.
// The caller must journal intent and independently verify ownership and the
// complete inspected configuration before starting or removing the container.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
)

const rootlessImage = "sha256:e4b2b083b10ff79a07e24bcc38d6352dff451bdc1e629002aeb7e5956b0b5db7"

var maskedPaths = []string{
	"/proc/asound", "/proc/acpi", "/proc/kcore", "/proc/keys", "/proc/latency_stats",
	"/proc/timer_list", "/proc/timer_stats", "/proc/sched_debug", "/proc/scsi",
	"/sys/firmware", "/sys/devices/virtual/powercap",
}
var readonlyPaths = []string{
	"/proc/bus", "/proc/fs", "/proc/irq", "/proc/sysrq-trigger", "/proc/sys/abi",
	"/proc/sys/debug", "/proc/sys/dev", "/proc/sys/fs", "/proc/sys/fscache",
	"/proc/sys/kernel", "/proc/sys/sunrpc", "/proc/sys/user", "/proc/sys/vm",
}
var hex32 = regexp.MustCompile(`^[0-9a-f]{32}$`)
var hex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)
var daemonID = regexp.MustCompile(`^[A-Za-z0-9_.:-]{1,128}$`)

type spec struct {
	Name      string `json:"name"`
	RunID     string `json:"run_id"`
	Nonce     string `json:"nonce"`
	Daemon    string `json:"daemon"`
	Network   string `json:"network"`
	Data      string `json:"data_volume"`
	Legacy    string `json:"legacy_volume"`
	Socket    string `json:"socket_volume"`
	Workspace string `json:"workspace_volume,omitempty"`
}

func (s spec) validate() error {
	if !strings.HasPrefix(s.RunID, "voiceci-") || !hex32.MatchString(strings.TrimPrefix(s.RunID, "voiceci-")) ||
		!hex32.MatchString(s.Nonce) || s.Name != s.RunID+"-container-"+s.Nonce || !hex64.MatchString(s.Network) ||
		!daemonID.MatchString(s.Daemon) {
		return errors.New("invalid rootless identity")
	}
	volume := regexp.MustCompile(`^` + regexp.QuoteMeta(s.RunID) + `-volume-[0-9a-f]{32}$`)
	if !volume.MatchString(s.Data) || !volume.MatchString(s.Legacy) || !volume.MatchString(s.Socket) ||
		s.Data == s.Legacy || s.Socket == s.Data || s.Socket == s.Legacy {
		return errors.New("invalid rootless volume identities")
	}
	if s.Workspace != "" && (!volume.MatchString(s.Workspace) || s.Workspace == s.Data ||
		s.Workspace == s.Legacy || s.Workspace == s.Socket) {
		return errors.New("invalid rootless workspace identity")
	}
	return nil
}

func buildConfig(s spec) (*container.Config, *container.HostConfig) {
	limit := int64(256)
	config := &container.Config{
		Image: rootlessImage, User: "rootless", Entrypoint: []string{"dockerd-entrypoint.sh"},
		Env: []string{"DOCKERD_ROOTLESS_ROOTLESSKIT_NET=none", "DOCKER_TLS_CERTDIR=",
			"DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS="},
		Cmd: []string{"dockerd", "--storage-driver=vfs", "--iptables=false", "--ip-masq=false",
			"--host=unix:///run/user/1000/docker.sock"},
		Labels: map[string]string{"voice.ci.run": s.RunID, "voice.ci.resource": s.Nonce, "voice.ci.kind": "container"},
	}
	host := &container.HostConfig{
		NetworkMode: container.NetworkMode(s.Network), CgroupnsMode: container.CgroupnsMode("private"),
		Resources:   container.Resources{Memory: 512 << 20, NanoCPUs: 1000000000, PidsLimit: &limit},
		MaskedPaths: append([]string(nil), maskedPaths...), ReadonlyPaths: append([]string(nil), readonlyPaths...),
		Mounts: []mount.Mount{
			{Type: mount.TypeVolume, Source: s.Data, Target: "/home/rootless/.local/share/docker"},
			{Type: mount.TypeVolume, Source: s.Legacy, Target: "/var/lib/docker"},
			{Type: mount.TypeVolume, Source: s.Socket, Target: "/run/user/1000"},
		},
	}
	if s.Workspace != "" {
		host.Mounts = append(host.Mounts, mount.Mount{Type: mount.TypeVolume, Source: s.Workspace, Target: "/workspace"})
	}
	return config, host
}

func localDockerHost(value string) bool {
	if strings.ContainsAny(value, "\x00\r\n") {
		return false
	}
	return strings.HasPrefix(value, "unix:///") ||
		value == "npipe:////./pipe/dockerDesktopLinuxEngine" || value == "npipe:////./pipe/docker_engine"
}

func parseSpec(input io.Reader) (spec, error) {
	var s spec
	raw, err := io.ReadAll(io.LimitReader(input, 4097))
	if err != nil || len(raw) > 4096 {
		return s, errors.New("invalid rootless request size")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	open, err := decoder.Token()
	if err != nil || open != json.Delim('{') {
		return s, errors.New("invalid rootless request")
	}
	fields := map[string]*string{"name": &s.Name, "run_id": &s.RunID, "nonce": &s.Nonce, "daemon": &s.Daemon,
		"network": &s.Network, "data_volume": &s.Data, "legacy_volume": &s.Legacy, "socket_volume": &s.Socket,
		"workspace_volume": &s.Workspace}
	seen := make(map[string]bool, len(fields))
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok || fields[name] == nil || seen[name] {
			return s, errors.New("invalid rootless request field")
		}
		seen[name] = true
		if err := decoder.Decode(fields[name]); err != nil {
			return s, errors.New("invalid rootless request value")
		}
	}
	closeToken, err := decoder.Token()
	if err != nil || closeToken != json.Delim('}') || (len(seen) != len(fields) && len(seen) != len(fields)-1) ||
		(len(seen) == len(fields)-1 && seen["workspace_volume"]) || (seen["workspace_volume"] && s.Workspace == "") {
		return s, errors.New("invalid rootless request object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return s, errors.New("invalid rootless request suffix")
	}
	if err := s.validate(); err != nil {
		return s, err
	}
	return s, nil
}

func run(input io.Reader, output io.Writer) (resultErr error) {
	if !localDockerHost(os.Getenv("DOCKER_HOST")) || os.Getenv("DOCKER_CONTEXT") != "" ||
		os.Getenv("DOCKER_TLS_VERIFY") != "" || os.Getenv("DOCKER_CERT_PATH") != "" || os.Getenv("DOCKER_API_VERSION") != "" {
		return errors.New("explicit local Docker socket is required")
	}
	s, err := parseSpec(input)
	if err != nil {
		return err
	}
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return errors.New("docker client unavailable")
	}
	defer func() {
		if err := cli.Close(); err != nil && resultErr == nil {
			resultErr = errors.New("docker client close failed")
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	info, err := cli.Info(ctx)
	if err != nil || info.ID != s.Daemon || info.OSType != "linux" {
		return errors.New("docker daemon identity differs")
	}
	config, host := buildConfig(s)
	created, err := cli.ContainerCreate(ctx, config, host, nil, nil, s.Name)
	if err != nil || !hex64.MatchString(created.ID) {
		return errors.New("docker create outcome uncertain")
	}
	_, err = fmt.Fprintln(output, created.ID)
	return err
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		os.Exit(2)
	}
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
