package spacemembership

import (
	"github.com/google/uuid"
	"testing"
)

func configValues() map[string]string {
	m := map[string]string{"BOT_SPACE_MEMBERSHIP_SOURCE_GENERATION": uuid.NewString(), "BOT_SPACE_MEMBERSHIP_SOURCE_KIND": "KNOWN_RETAINED"}
	for _, p := range []string{"BOT_SPACE_MEMBERSHIP_", "BOT_INSTALL_ROLE_", "BOT_INSTALL_CHAT_", "BOT_INSTALL_MESSAGING_"} {
		for k, v := range map[string]string{"GRPC_ADDR": "authority.internal:9443", "TLS_CA_FILE": "ca.pem", "TLS_CERT_FILE": "client.pem", "TLS_KEY_FILE": "key.pem"} {
			m[p+k] = v
		}
	}
	return m
}
func TestMembershipConfigNeverFallsBackToRawEndpoints(t *testing.T) {
	for _, kind := range []string{"absent", "blank", "partial", "unknown_source", "bad_generation", "valid"} {
		t.Run(kind, func(t *testing.T) {
			values := configValues()
			switch kind {
			case "absent":
				values = map[string]string{"SPACE_GRPC_ADDR": "insecure:9090"}
			case "blank":
				values["BOT_SPACE_MEMBERSHIP_GRPC_ADDR"] = ""
			case "partial":
				delete(values, "BOT_INSTALL_ROLE_TLS_CA_FILE")
			case "unknown_source":
				values["BOT_SPACE_MEMBERSHIP_SOURCE_KIND"] = "RESTORED"
			case "bad_generation":
				values["BOT_SPACE_MEMBERSHIP_SOURCE_GENERATION"] = uuid.Nil.String()
			}
			cfg, on, err := LoadConfig(func(k string) (string, bool) { v, ok := values[k]; return v, ok })
			if kind == "absent" {
				if on || err != nil {
					t.Fatal("ordinary endpoint enabled protected mode")
				}
				return
			}
			if kind != "valid" {
				if !on || err == nil {
					t.Fatal("partial/unknown configuration accepted")
				}
				return
			}
			if err != nil || !on || cfg.Space.ServerName != "authority.internal" {
				t.Fatal("valid config rejected", err)
			}
			if _, err = cfg.Space.TLS(); err == nil {
				t.Fatal("missing trust/client certificate accepted")
			}
		})
	}
}
