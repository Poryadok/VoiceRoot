package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootlessRequestRejectsTrailingOversizedAndUnknownInput(t *testing.T) {
	raw, err := json.Marshal(validSpec())
	require.NoError(t, err)
	_, err = parseSpec(strings.NewReader(string(raw)))
	require.NoError(t, err)
	for _, input := range []string{string(raw) + `{}`, string(raw) + strings.Repeat(" ", 4097),
		strings.TrimSuffix(string(raw), "}") + `,"unknown":true}`,
		strings.TrimSuffix(string(raw), "}") + `,"name":"` + validSpec().Name + `"}`} {
		_, err = parseSpec(strings.NewReader(input))
		require.Error(t, err)
	}
}

func TestDockerHostMustBeAnExplicitLocalSocket(t *testing.T) {
	for _, host := range []string{"", "tcp://127.0.0.1:2375", "https://example.invalid", "npipe:////./pipe/other", "unix:///tmp/socket\nother"} {
		t.Run(host, func(t *testing.T) { require.False(t, localDockerHost(host)) })
	}
	for _, host := range []string{"unix:///var/run/docker.sock", "npipe:////./pipe/dockerDesktopLinuxEngine", "npipe:////./pipe/docker_engine"} {
		t.Run(host, func(t *testing.T) { require.True(t, localDockerHost(host)) })
	}
}

func TestRootlessDaemonUsesOwnedUnixSocketWithoutTCP(t *testing.T) {
	config, host := buildConfig(validSpec())
	require.Equal(t, []string{"dockerd", "--storage-driver=vfs", "--iptables=false", "--ip-masq=false",
		"--host=unix:///run/user/1000/docker.sock"}, []string(config.Cmd))
	require.Contains(t, config.Env, "DOCKERD_ROOTLESS_ROOTLESSKIT_FLAGS=")
	require.Len(t, host.Mounts, 3)
	require.Equal(t, "/run/user/1000", host.Mounts[2].Target)
}

func TestRootlessWorkspaceIsOptionalOwnedAndDistinct(t *testing.T) {
	s := validSpec()
	s.Workspace = s.RunID + "-volume-" + strings.Repeat("1", 32)
	require.NoError(t, s.validate())
	raw, err := json.Marshal(s)
	require.NoError(t, err)
	parsed, err := parseSpec(strings.NewReader(string(raw)))
	require.NoError(t, err)
	_, host := buildConfig(parsed)
	require.Len(t, host.Mounts, 4)
	require.Equal(t, "/workspace", host.Mounts[3].Target)
	require.Equal(t, s.Workspace, host.Mounts[3].Source)
	for _, invalid := range []string{"foreign", s.Data, s.Legacy, s.Socket} {
		s.Workspace = invalid
		require.Error(t, s.validate())
	}
}

func validSpec() spec {
	run := "voiceci-" + strings.Repeat("a", 32)
	nonce := strings.Repeat("b", 32)
	return spec{Name: run + "-container-" + nonce, RunID: run, Nonce: nonce, Daemon: "daemon-a",
		Network: strings.Repeat("c", 64), Data: run + "-volume-" + strings.Repeat("d", 32),
		Legacy: run + "-volume-" + strings.Repeat("e", 32), Socket: run + "-volume-" + strings.Repeat("f", 32)}
}

func TestRootlessContainerProfileIsBoundedAndPrivate(t *testing.T) {
	s := validSpec()
	require.NoError(t, s.validate())
	config, host := buildConfig(s)
	require.Equal(t, rootlessImage, config.Image)
	require.Equal(t, []string{"dockerd-entrypoint.sh"}, []string(config.Entrypoint))
	require.Equal(t, s.RunID, config.Labels["voice.ci.run"])
	require.False(t, host.Privileged)
	require.Empty(t, host.Binds)
	require.Empty(t, host.PortBindings)
	require.Empty(t, host.CapAdd)
	require.Empty(t, host.SecurityOpt)
	require.Equal(t, "private", string(host.CgroupnsMode))
	require.Equal(t, s.Network, string(host.NetworkMode))
	require.ElementsMatch(t, maskedPaths, host.MaskedPaths)
	require.ElementsMatch(t, readonlyPaths, host.ReadonlyPaths)
	require.Len(t, host.Mounts, 3)
	require.Equal(t, int64(512<<20), host.Memory)
	require.Equal(t, int64(1000000000), host.NanoCPUs)
	require.Equal(t, int64(256), *host.PidsLimit)
}

func TestRootlessSpecRejectsForeignOrAmbiguousResources(t *testing.T) {
	for name, mutate := range map[string]func(*spec){
		"name":           func(s *spec) { s.Name = "foreign" },
		"run":            func(s *spec) { s.RunID = "voiceci-other" },
		"nonce":          func(s *spec) { s.Nonce = strings.Repeat("z", 32) },
		"network":        func(s *spec) { s.Network = "host" },
		"daemon":         func(s *spec) { s.Daemon = "" },
		"data":           func(s *spec) { s.Data = "foreign-volume" },
		"legacy":         func(s *spec) { s.Legacy = s.Data },
		"socket":         func(s *spec) { s.Socket = s.Legacy },
		"socket-foreign": func(s *spec) { s.Socket = "foreign-volume" },
		"socket-empty":   func(s *spec) { s.Socket = "" },
	} {
		t.Run(name, func(t *testing.T) {
			s := validSpec()
			mutate(&s)
			require.Error(t, s.validate())
		})
	}
}
