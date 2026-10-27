package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestStartupPeerServiceDoesNotExposeOrdinaryUnreadyRoutes(t *testing.T) {
	root := repoRootFromComposeTest(t)
	raw, err := os.ReadFile(filepath.Join(root, "deploy/staging/services.yaml"))
	require.NoError(t, err)
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	objects := map[string]map[string]any{}
	for {
		var doc map[string]any
		err := decoder.Decode(&doc)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		meta := doc["metadata"].(map[string]any)
		objects[doc["kind"].(string)+"/"+meta["name"].(string)] = doc
	}
	peer := objects["Service/voice-user-peer"]["spec"].(map[string]any)
	require.Equal(t, "ClusterIP", peer["type"])
	require.Equal(t, true, peer["publishNotReadyAddresses"])
	require.Equal(t, map[string]any{"app": "voice-user"}, peer["selector"])
	require.Equal(t, []any{map[string]any{"name": "grpc", "port": 9090, "targetPort": 9090}}, peer["ports"])
	for _, name := range []string{"voice-user", "voice-space"} {
		ordinary := objects["Service/"+name]["spec"].(map[string]any)
		require.NotEqual(t, true, ordinary["publishNotReadyAddresses"], name)
	}
	peerConsumers := []string{}
	for key, doc := range objects {
		if doc["kind"] != "Deployment" {
			continue
		}
		spec := doc["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
		for _, entry := range spec["containers"].([]any) {
			container := entry.(map[string]any)
			for _, e := range optionalYAMLList(container["env"]) {
				env := e.(map[string]any)
				if env["value"] == "voice-user-peer:9090" {
					peerConsumers = append(peerConsumers, key+"/"+container["name"].(string)+"/"+env["name"].(string))
				}
			}
		}
	}
	require.Equal(t, []string{"Deployment/voice-space/space/USER_GRPC_ADDR"}, peerConsumers)
	config, err := os.ReadFile(filepath.Join(root, "deploy/staging/configmap-app.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(config), "USER_GRPC_ADDR: \"voice-user:9090\"")
	require.NotContains(t, string(config), "voice-user-peer")
}

func optionalYAMLList(value any) []any {
	if value == nil {
		return nil
	}
	return value.([]any)
}
