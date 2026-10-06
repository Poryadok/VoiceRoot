package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVerifyActorPrivatePipe(t *testing.T) {
	f := newActorFixture(t)
	credentials, account, operator, actorSHA, accountSHA := f.inputs(t)
	request := map[string]any{"schema": "voice-nats-existing-actor-verification-v1", "credentials": string(credentials), "account_jwt": account, "operator_jwt": operator, "actor_sha256": actorSHA, "account_sha256": accountSHA}
	raw, _ := json.Marshal(request)
	var output bytes.Buffer
	if verifyActorPipe(bytes.NewReader(raw), &output) != nil {
		t.Fatal("private verifier rejected original synthetic identity")
	}
	var receipt map[string]any
	if json.Unmarshal(output.Bytes(), &receipt) != nil || receipt["signed_chain_verified"] != true {
		t.Fatal("private signed receipt missing")
	}
	for _, private := range []string{string(credentials), account, operator, "SEED"} {
		if strings.Contains(output.String(), private) {
			t.Fatal("private transport returned credential material")
		}
	}
	request["signer_seed"] = "forbidden-even-synthetic"
	raw, _ = json.Marshal(request)
	output.Reset()
	if verifyActorPipe(bytes.NewReader(raw), &output) == nil || output.Len() != 0 {
		t.Fatal("verification accepted signer field")
	}
	delete(request, "signer_seed")
	request["actor_sha256"] = strings.Repeat("0", 64)
	raw, _ = json.Marshal(request)
	if verifyActorPipe(bytes.NewReader(raw), &output) == nil || output.Len() != 0 {
		t.Fatal("private verifier accepted wrong actor")
	}
}
