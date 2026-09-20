package principal

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Static rollout contract: production configuration must ship with the
// application cutover, otherwise a seemingly green code-only change would
// leave protected listeners unreachable or use an unmounted private key.
func TestSocialPrivacyPrincipalRollout_StagingAndProdDeclareProtectedResources(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	for _, environment := range []string{"staging", "prod"} {
		t.Run(environment, func(t *testing.T) {
			services, err := os.ReadFile(filepath.Join(root, "deploy", environment, "services.yaml"))
			require.NoError(t, err)
			manifest := string(services)
			deployments := map[string]map[string]any{}
			decoder := yaml.NewDecoder(bytes.NewReader(services))
			for {
				var object map[string]any
				err := decoder.Decode(&object)
				if err == io.EOF {
					break
				}
				require.NoError(t, err)
				if object["kind"] == "Deployment" {
					name := object["metadata"].(map[string]any)["name"].(string)
					deployments[name] = object
				}
			}
			for _, service := range []string{"social", "user", "space"} {
				object := deployments["voice-"+service]
				require.NotNil(t, object)
				pod := object["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
				container := pod["containers"].([]any)[0].(map[string]any)
				env := map[string]any{}
				for _, raw := range container["env"].([]any) {
					entry := raw.(map[string]any)
					env[entry["name"].(string)] = entry
				}
				prefix := strings.ToUpper(service) + "_PRINCIPAL_"
				for _, suffix := range []string{"TLS_CERT_FILE", "TLS_KEY_FILE"} {
					require.Contains(t, env, prefix+suffix)
				}
				require.NotEmpty(t, pod["volumes"], "TLS secrets must be mounted")
				for _, raw := range container["volumeMounts"].([]any) {
					require.Equal(t, true, raw.(map[string]any)["readOnly"])
				}
				if service == "social" {
					for _, name := range []string{"SOCIAL_PRINCIPAL_SIGNING_KEYS_DIR", "SOCIAL_PRINCIPAL_ACTIVE_KID", "USER_PRINCIPAL_GRPC_ADDR", "SPACE_PRINCIPAL_GRPC_ADDR"} {
						require.Contains(t, env, name)
					}
				} else {
					for _, name := range []string{prefix + "GRPC_LISTEN", prefix + "REPLAY_REDIS_ADDR", "S2S_JWKS_URLS_JSON", "S2S_JWKS_CA_FILE"} {
						require.Contains(t, env, name)
					}
				}
			}

			for _, required := range []string{
				"SOCIAL_PRINCIPAL_SIGNING_KEYS_DIR",
				"SOCIAL_PRINCIPAL_ACTIVE_KID",
				"USER_PRINCIPAL_GRPC_LISTEN",
				"SPACE_PRINCIPAL_GRPC_LISTEN",
				"USER_PRINCIPAL_REPLAY_REDIS_ADDR",
				"SPACE_PRINCIPAL_REPLAY_REDIS_ADDR",
				"containerPort: 9091",
				"readOnly: true",
			} {
				require.True(t, strings.Contains(manifest, required), "missing rollout declaration %q", required)
			}
		})
	}

	policy, err := os.ReadFile(filepath.Join(root, "deploy", "templates", "network-policy-social-privacy-principal.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(policy), "voice-social")
	require.Contains(t, string(policy), "9091")
	require.Contains(t, string(policy), "9090", "unrelated profile/account lookups must survive privacy cutover")
}

func TestSocialPrivacyComposeKeepsSigningKeysIssuerOwned(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	require.NoError(t, err)
	var compose struct {
		Services map[string]struct {
			Environment map[string]string `yaml:"environment"`
			Volumes     []string          `yaml:"volumes"`
			DependsOn   map[string]struct {
				Condition string `yaml:"condition"`
			} `yaml:"depends_on"`
		} `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(data, &compose))
	for name, service := range compose.Services {
		for _, volume := range service.Volumes {
			if strings.HasPrefix(volume, "social_principal_keys:") {
				require.Contains(t, []string{"social-principal-init", "social"}, name, "private issuer keys leaked to another service")
			}
		}
	}
	for _, name := range []string{"social", "user", "space"} {
		service := compose.Services[name]
		require.Equal(t, "service_completed_successfully", service.DependsOn["social-principal-init"].Condition)
		for _, volume := range service.Volumes {
			require.True(t, strings.HasSuffix(volume, ":ro"))
		}
		prefix := strings.ToUpper(name) + "_PRINCIPAL_"
		require.NotEmpty(t, service.Environment[prefix+"TLS_CERT_FILE"])
		require.NotEmpty(t, service.Environment[prefix+"TLS_KEY_FILE"])
		if name != "social" {
			require.Equal(t, ":9091", service.Environment[prefix+"GRPC_LISTEN"])
			require.Contains(t, service.Environment["S2S_JWKS_URLS_JSON"], "https://social:8443/")
		}
	}
}

func TestFileUserPrincipalRollout_DeclaresOnlyDedicatedPortsAndTrust(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	for _, environment := range []string{"staging", "prod"} {
		data, err := os.ReadFile(filepath.Join(root, "deploy", environment, "services.yaml"))
		require.NoError(t, err)
		manifest := string(data)
		for _, required := range []string{
			"USER_FILE_PRINCIPAL_GRPC_LISTEN", "USER_FILE_PRINCIPAL_REPLAY_REDIS_ADDR",
			"FILE_PRINCIPAL_SIGNING_KEYS_DIR", "FILE_PRINCIPAL_ACTIVE_KID",
			"FILE_PRINCIPAL_JWKS_LISTEN", "USER_FILE_PRINCIPAL_GRPC_ADDR",
			"containerPort: 9092", "containerPort: 8443", "file-principal-grpc", "principal-jwks",
			"voice-file-principal-signing", "voice-file-principal-tls", "voice-user-file-principal-tls",
			"https://voice-file:8443/.well-known/jwks.json", "readOnly: true",
		} {
			require.Contains(t, manifest, required, "%s must declare %s", environment, required)
		}
	}
	policy, err := os.ReadFile(filepath.Join(root, "deploy", "templates", "network-policy-file-user-principal.yaml"))
	require.NoError(t, err)
	for _, required := range []string{"voice-file", "voice-user", "9092", "8443", "voice-social", "9091", "9090"} {
		require.Contains(t, string(policy), required)
	}
}

func TestFileUserPrincipalComposeKeepsFileSigningKeysIssuerOwned(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	data, err := os.ReadFile(filepath.Join(root, "docker-compose.yml"))
	require.NoError(t, err)
	var compose struct {
		Services map[string]struct {
			Environment map[string]string `yaml:"environment"`
			Volumes     []string          `yaml:"volumes"`
		} `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal(data, &compose))
	for name, service := range compose.Services {
		for _, volume := range service.Volumes {
			if strings.HasPrefix(volume, "file_principal_keys:") {
				require.Contains(t, []string{"social-principal-init", "file"}, name, "File private keys leaked to %s", name)
			}
		}
	}
	file := compose.Services["file"]
	for _, name := range []string{"FILE_PRINCIPAL_SIGNING_KEYS_DIR", "FILE_PRINCIPAL_ACTIVE_KID", "FILE_PRINCIPAL_JWKS_LISTEN", "USER_FILE_PRINCIPAL_GRPC_ADDR", "USER_FILE_PRINCIPAL_TLS_CA_FILE", "USER_FILE_PRINCIPAL_TLS_SERVER_NAME"} {
		require.NotEmpty(t, file.Environment[name])
	}
	user := compose.Services["user"]
	for _, name := range []string{"USER_FILE_PRINCIPAL_GRPC_LISTEN", "USER_FILE_PRINCIPAL_TLS_CERT_FILE", "USER_FILE_PRINCIPAL_TLS_KEY_FILE", "USER_FILE_PRINCIPAL_REPLAY_REDIS_ADDR"} {
		require.NotEmpty(t, user.Environment[name])
	}
	require.Contains(t, user.Environment["S2S_JWKS_URLS_JSON"], "https://file:8443/")
}
