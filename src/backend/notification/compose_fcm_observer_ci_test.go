//go:build voice_compose_fcm_diagnostic

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const (
	diagnosticTestMessage = "11111111-1111-4111-8111-111111111111"
	diagnosticTestChat    = "22222222-2222-4222-8222-222222222222"
	diagnosticTestSender  = "33333333-3333-4333-8333-333333333333"
	diagnosticTestTarget  = "44444444-4444-4444-8444-444444444444"
)

func writeDiagnosticControl(t *testing.T, path, messageID string) {
	t.Helper()
	data, err := json.Marshal(composeFcmControl{MessageID: messageID, ChatID: diagnosticTestChat, SenderProfileID: diagnosticTestSender, RecipientID: diagnosticTestTarget})
	if err != nil {
		t.Fatal("control encoding failed")
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal("control write failed")
	}
}

func TestComposeFcmObserverCorrelatesBothResponseOrdersAndEventRetries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	writeDiagnosticControl(t, path, "")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	first := o.begin("event-a", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender)
	if first == nil {
		t.Fatal("pre-response event was not recorded")
	}
	if retry := o.begin("event-a", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender); retry != first || first.attempts != 2 {
		t.Fatal("same message and event pair was not classified as retry")
	}
	if distinct := o.begin("event-b", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender); distinct == nil || distinct == first {
		t.Fatal("distinct event identity was not retained separately")
	}
	writeDiagnosticControl(t, path, diagnosticTestMessage)
	o.mu.Lock()
	match, ambiguous := o.candidateForMessage(diagnosticTestMessage)
	o.mu.Unlock()
	if match != nil || !ambiguous {
		t.Fatal("different event identities for the response message were not ambiguous")
	}

	writeDiagnosticControl(t, path, diagnosticTestMessage)
	responseFirst := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	if responseFirst.begin("event-c", diagnosticTestMessage, diagnosticTestChat, diagnosticTestSender) == nil {
		t.Fatal("response-before-consumer event was not recorded")
	}
	responseFirst.mu.Lock()
	match, ambiguous = responseFirst.candidateForMessage(diagnosticTestMessage)
	responseFirst.mu.Unlock()
	if match == nil || ambiguous || match.eventID != "event-c" {
		t.Fatal("response-before-consumer event did not correlate uniquely")
	}
}

func TestComposeFcmObserverCapacityFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.json")
	writeDiagnosticControl(t, path, "")
	o := &composeFcmObserver{path: path, candidates: make(map[string]*composeFcmTrace)}
	for i := 0; i < composeFcmCandidateLimit; i++ {
		messageID := diagnosticTestMessage[:35] + string(rune('a'+i))
		if o.begin("event-"+string(rune('a'+i)), messageID, diagnosticTestChat, diagnosticTestSender) == nil {
			t.Fatal("candidate rejected before capacity")
		}
	}
	if o.begin("overflow", "55555555-5555-4555-8555-555555555555", diagnosticTestChat, diagnosticTestSender) != nil || !o.overflow {
		t.Fatal("capacity exhaustion was not recorded as fail-closed overflow")
	}
}
