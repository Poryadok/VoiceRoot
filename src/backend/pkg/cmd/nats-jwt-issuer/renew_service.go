package main

import (
	"bytes"
	"encoding/json"
	"io"
	"slices"
	"strings"

	"github.com/nats-io/jwt/v2"
)

// These are the only service additions this explicit operator transition can
// issue. ACK tails are restricted to the named service-owned consumers.
var serviceRenewSubjects = map[string][]string{
	"role":  {"role.voice_policy_invalidated"},
	"space": {"space.voice_room_access_invalidated"},
	"voice": {
		"voice.participant_joined",
		"voice.participant_left",
		"$JS.API.CONSUMER.INFO.chat_events.voice_space_media_chat",
		"$JS.ACK.chat_events.voice_space_media_chat.>",
		"$JS.API.CONSUMER.INFO.role_events.voice_space_media_role",
		"$JS.ACK.role_events.voice_space_media_role.>",
	},
	"messaging": {
		"$JS.API.CONSUMER.INFO.file_events.messaging_music_metadata",
		"$JS.ACK.file_events.messaging_music_metadata.>",
	},
}

func containsServiceSubject(subjects []string, subject string) bool {
	return slices.Contains(subjects, subject)
}

func sameServiceSubjects(a, b []string) bool {
	left, right := slices.Clone(a), slices.Clone(b)
	slices.Sort(left)
	slices.Sort(right)
	return slices.Equal(left, right) && len(slices.Compact(left)) == len(a) && len(slices.Compact(right)) == len(b)
}

// Source may list concrete inboxes already covered by the same existing owned
// tail. Compare that authority without adding the redundant claims to the JWT.
// Other differences (including a missing tail or unrelated allow) still veto.
func sameServiceSubscriptions(a, b []string, service string) bool {
	compact := func(subjects []string) []string {
		prefix := "_INBOX.voice." + service + "."
		if !slices.Contains(subjects, prefix+">") {
			return subjects
		}
		out := make([]string, 0, len(subjects))
		for _, subject := range subjects {
			if strings.HasPrefix(subject, prefix) && len(subject) > len(prefix) && !strings.ContainsAny(subject, "*>") {
				continue
			}
			out = append(out, subject)
		}
		return out
	}
	return sameServiceSubjects(a, a) && sameServiceSubjects(b, b) && sameServiceSubjects(compact(a), compact(b))
}

// NATS > consumes at least one token. A concrete deny inside an allowed ACK
// tail is an overlap too; matching a deny against the literal ">" is unsafe.
func serviceSubjectsOverlap(a, b string) bool {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] == ">" || right[i] == ">" {
			return true
		}
		if left[i] != right[i] && left[i] != "*" && right[i] != "*" {
			return false
		}
	}
	return len(left) == len(right)
}

func renewExistingService(old []byte, accountToken, operatorToken string, signerSeed []byte, actorHash, accountHash, service string, target grant) ([]byte, error) {
	subjects, ok := serviceRenewSubjects[service]
	if !ok || !validGrant(target, service, false) {
		return nil, renewalFailure()
	}
	if _, err := verifyExistingActor(old, accountToken, operatorToken, actorHash, accountHash); err != nil {
		return nil, renewalFailure()
	}
	token, _ := jwt.ParseDecoratedJWT(old)
	u, err := jwt.DecodeUserClaims(token)
	if err != nil || u.Name != "voice-"+service || len(u.Pub.Allow) == 0 || !sameServiceSubscriptions(u.Sub.Allow, target.Subscribe, service) {
		return nil, renewalFailure()
	}
	for _, subject := range target.Subscribe {
		for _, deny := range u.Sub.Deny {
			if serviceSubjectsOverlap(deny, subject) {
				return nil, renewalFailure()
			}
		}
	}
	result := slices.Clone([]string(u.Pub.Allow))
	for _, subject := range subjects {
		for _, deny := range u.Pub.Deny {
			if serviceSubjectsOverlap(deny, subject) {
				return nil, renewalFailure()
			}
		}
		if !containsServiceSubject(result, subject) {
			result = append(result, subject)
		}
	}
	if !sameServiceSubjects(result, target.Publish) {
		return nil, renewalFailure()
	}
	// Always validate signer custody/binding, including already-current inputs.
	renewed, err := renewExistingSubjects(old, accountToken, operatorToken, signerSeed, actorHash, accountHash, subjects)
	if err != nil {
		return nil, renewalFailure()
	}
	if _, err := verifyExistingActor(renewed, accountToken, operatorToken, actorHash, accountHash); err != nil {
		return nil, renewalFailure()
	}
	if sameServiceSubjects(u.Pub.Allow, result) {
		return bytes.Clone(old), nil
	}
	return renewed, nil
}

func renewServicePipe(input io.Reader, output io.Writer) error {
	raw, err := io.ReadAll(io.LimitReader(input, 1048577))
	if err != nil || len(raw) == 0 || len(raw) > 1048576 {
		return renewalFailure()
	}
	defer clear(raw)
	var request struct {
		Schema     string   `json:"schema"`
		Old        string   `json:"old_creds"`
		Account    string   `json:"account_jwt"`
		Operator   string   `json:"operator_jwt"`
		Seed       string   `json:"signer_seed"`
		ActorSHA   string   `json:"actor_sha256"`
		AccountSHA string   `json:"account_sha256"`
		Service    string   `json:"service"`
		Publish    []string `json:"publish"`
		Subscribe  []string `json:"subscribe"`
		NoResponse bool     `json:"no_response"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || request.Schema != "voice-nats-existing-service-renewal-v1" {
		return renewalFailure()
	}
	result, err := renewExistingService([]byte(request.Old), request.Account, request.Operator, []byte(request.Seed), request.ActorSHA, request.AccountSHA, request.Service, grant{request.Publish, request.Subscribe, request.NoResponse})
	if err != nil {
		return renewalFailure()
	}
	defer clear(result)
	n, err := output.Write(result)
	if err != nil || n != len(result) {
		return renewalFailure()
	}
	return nil
}
