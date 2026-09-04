package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const canonicalAuthGRPCAddr = "voice-auth:9090"

// TestChatAuthGRPCAddrDeploymentWiring keeps the non-secret Auth gRPC
// dependency available to Chat in every supported runtime.
func TestChatAuthGRPCAddrDeploymentWiring(t *testing.T) {
	t.Parallel()

	root := chatRepoRoot(t)
	for _, environment := range []string{"staging", "prod"} {
		environment := environment
		t.Run(environment, func(t *testing.T) {
			configMap := readKubernetesManifest(t, filepath.Join(root, "deploy", environment, "configmap-app.yaml"))
			require.Equal(t, "ConfigMap", configMap.Kind)
			require.Equal(t, "voice-app-config", configMap.Metadata.Name)
			require.Equal(t, canonicalAuthGRPCAddr, configMap.Data["AUTH_GRPC_ADDR"])

			chat := findChatDeployment(t, readKubernetesManifests(t, filepath.Join(root, "deploy", environment, "services.yaml")))
			container := findChatContainer(t, chat)
			require.Contains(t, container.ConfigMapRefs(), "voice-app-config")
			require.Empty(t, container.SecretRefs(), "Chat envFrom must not allow a Secret to override AUTH_GRPC_ADDR")
			for _, env := range container.Env {
				if env.Name == "AUTH_GRPC_ADDR" {
					require.Nil(t, env.ValueFrom.SecretKeyRef, "AUTH_GRPC_ADDR must remain a ConfigMap literal, not a secret")
				}
			}
		})
	}

	compose := readComposeSpec(t, filepath.Join(root, "docker-compose.yml"))
	require.Equal(t, "auth:9090", compose.Services["chat"].Environment["AUTH_GRPC_ADDR"])
}

func chatRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

type kubernetesManifest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Data map[string]string `yaml:"data"`
	Spec struct {
		Template struct {
			Spec struct {
				Containers []kubernetesContainer `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type kubernetesContainer struct {
	Name    string             `yaml:"name"`
	Env     []kubernetesEnvVar `yaml:"env"`
	EnvFrom []struct {
		ConfigMapRef *kubernetesKeyRef `yaml:"configMapRef"`
		SecretRef    *kubernetesKeyRef `yaml:"secretRef"`
	} `yaml:"envFrom"`
}

func (container kubernetesContainer) ConfigMapRefs() []string {
	refs := make([]string, 0, len(container.EnvFrom))
	for _, source := range container.EnvFrom {
		if source.ConfigMapRef != nil {
			refs = append(refs, source.ConfigMapRef.Name)
		}
	}
	return refs
}

func (container kubernetesContainer) SecretRefs() []string {
	refs := make([]string, 0, len(container.EnvFrom))
	for _, source := range container.EnvFrom {
		if source.SecretRef != nil {
			refs = append(refs, source.SecretRef.Name)
		}
	}
	return refs
}

type kubernetesEnvVar struct {
	Name      string `yaml:"name"`
	ValueFrom struct {
		SecretKeyRef *kubernetesKeyRef `yaml:"secretKeyRef"`
	} `yaml:"valueFrom"`
}

type kubernetesKeyRef struct {
	Name string `yaml:"name"`
}

func readKubernetesManifest(t *testing.T, path string) kubernetesManifest {
	t.Helper()
	manifests := readKubernetesManifests(t, path)
	require.Len(t, manifests, 1)
	return manifests[0]
}

func readKubernetesManifests(t *testing.T, path string) []kubernetesManifest {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	var manifests []kubernetesManifest
	for {
		var manifest kubernetesManifest
		err := decoder.Decode(&manifest)
		if err == io.EOF {
			return manifests
		}
		require.NoError(t, err)
		if manifest.Kind != "" {
			manifests = append(manifests, manifest)
		}
	}
}

func findChatDeployment(t *testing.T, manifests []kubernetesManifest) kubernetesManifest {
	t.Helper()
	for _, manifest := range manifests {
		if manifest.Kind == "Deployment" && manifest.Metadata.Name == "voice-chat" {
			return manifest
		}
	}
	t.Fatalf("voice-chat deployment is missing")
	return kubernetesManifest{}
}

func findChatContainer(t *testing.T, deployment kubernetesManifest) kubernetesContainer {
	t.Helper()
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "chat" {
			return container
		}
	}
	t.Fatalf("chat container is missing from voice-chat deployment")
	return kubernetesContainer{}
}

type composeSpec struct {
	Services map[string]composeService `yaml:"services"`
}

type composeService struct {
	Environment map[string]string `yaml:"environment"`
}

func readComposeSpec(t *testing.T, path string) composeSpec {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var compose composeSpec
	require.NoError(t, yaml.Unmarshal(raw, &compose))
	return compose
}
