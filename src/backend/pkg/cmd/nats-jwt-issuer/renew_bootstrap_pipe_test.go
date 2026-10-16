package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRenewBootstrapPrivatePipeCompletesAndRejectsUnknownInput(t *testing.T) {
	old, at, ot, seed, actor, account, _, _, _ := renewalFixture(t)
	request := map[string]string{"schema": "voice-nats-existing-bootstrap-renewal-v1", "old_creds": string(old), "account_jwt": at, "operator_jwt": ot, "signer_seed": string(seed), "actor_sha256": actor, "account_sha256": account}
	raw, _ := json.Marshal(request)
	var output bytes.Buffer
	if err := renewBootstrapPipe(bytes.NewReader(raw), &output); err != nil {
		t.Fatal(err)
	}
	if output.Len() == 0 {
		t.Fatal("private result absent")
	}
	request["arbitrary_permission"] = "$JS.API.>"
	raw, _ = json.Marshal(request)
	output.Reset()
	if err := renewBootstrapPipe(bytes.NewReader(raw), &output); err == nil || output.Len() != 0 {
		t.Fatal("unknown authority accepted")
	}
}
func TestRenewBootstrapPrivatePipeRejectsTrailingAndOversize(t *testing.T) {
	for _, raw := range []string{`{} {}`, strings.Repeat(" ", 1048577)} {
		var output bytes.Buffer
		if err := renewBootstrapPipe(strings.NewReader(raw), &output); err == nil || output.Len() != 0 {
			t.Fatal("invalid request emitted private material")
		}
	}
}
