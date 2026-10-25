package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nats-io/jwt/v2"
	"gopkg.in/yaml.v3"
)

func TestAuthObservationsCanonicalPublishAndStreamCapture(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..")
	acl, err := loadACL(filepath.Join(root, "deploy/nats/acl-intent.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	subjects := []string{"user.registered", "user.logged_in", "user.logged_out", "user.2fa_enabled"}
	for _, subject := range subjects {
		if !slices.Contains(acl.Services["auth"].Publish, subject) {
			t.Errorf("Auth cannot publish observation %s", subject)
		}
	}
	for _, subject := range acl.Services["auth"].Publish {
		if strings.ContainsAny(subject, "*>") && !strings.HasPrefix(subject, "$JS.ACK.") {
			t.Errorf("broad Auth publish grant %s", subject)
		}
	}
	destination := filepath.Join(t.TempDir(), "credentials")
	if err := generate(destination, acl); err != nil {
		t.Fatal(err)
	}
	claims, err := jwt.DecodeUserClaims(credsJWT(t, mustRead(t, filepath.Join(destination, "creds/auth.creds"))))
	if err != nil {
		t.Fatal(err)
	}
	for _, subject := range subjects {
		if !slices.Contains(claims.Pub.Allow, subject) {
			t.Errorf("signed Auth JWT omits %s", subject)
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "deploy/nats/jetstream-publisher-streams.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Streams []struct {
			Name     string   `yaml:"name"`
			Subjects []string `yaml:"subjects"`
		} `yaml:"streams"`
	}
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	var streamSubjects []string
	for _, stream := range manifest.Streams {
		if stream.Name == "user_events" {
			streamSubjects = stream.Subjects
		}
	}
	for _, subject := range subjects {
		if !slices.Contains(streamSubjects, subject) {
			t.Errorf("user_events does not capture %s", subject)
		}
	}
	for _, path := range []string{"docker/nats/realtime-bootstrap.sh", "docker/nats/search-bootstrap.sh", "deploy/templates/nats-realtime-bootstrap.yaml", "deploy/templates/nats-search-bootstrap.yaml", "deploy/templates/nats-analytics-chat-bootstrap.yaml"} {
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		var declared []string
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 2 && fields[0] == "stream" && fields[1] == "user_events" {
				declared = fields[2:]
			}
		}
		for _, subject := range subjects {
			if !slices.Contains(declared, subject) {
				t.Errorf("%s does not capture %s", path, subject)
			}
		}
	}
}
