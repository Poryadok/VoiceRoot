package spacemembership

import (
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"testing"
)

func TestMembershipConfigRetainedSourceFile(t *testing.T) {
	id := uuid.NewString()
	for _, test := range []struct {
		name, body            string
		missing, both, wantOK bool
	}{
		{name: "valid", body: id + "\n", wantOK: true},
		{name: "exact", body: id, wantOK: true},
		{name: "empty"}, {name: "missing", missing: true},
		{name: "extra", body: id + "\nextra"}, {name: "space", body: " " + id},
		{name: "nil", body: uuid.Nil.String()}, {name: "ambiguous", body: id, both: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			values := configValues()
			delete(values, "BOT_SPACE_MEMBERSHIP_SOURCE_GENERATION")
			path := filepath.Join(t.TempDir(), "source.uuid")
			if !test.missing {
				if err := os.WriteFile(path, []byte(test.body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			values["BOT_SPACE_MEMBERSHIP_SOURCE_FILE"] = path
			if test.both {
				values["BOT_SPACE_MEMBERSHIP_SOURCE_GENERATION"] = id
			}
			cfg, on, err := LoadConfig(func(k string) (string, bool) { v, ok := values[k]; return v, ok })
			if !on || (err == nil) != test.wantOK {
				t.Fatalf("on=%v err=%v", on, err)
			}
			if test.wantOK && cfg.SourceGeneration.String() != id {
				t.Fatal("source identity changed")
			}
		})
	}
}

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
