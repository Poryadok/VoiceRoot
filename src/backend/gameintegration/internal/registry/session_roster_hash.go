package registry

import (
	"crypto/sha256"
	"encoding/json"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/google/uuid"
)

// sessionRosterHash binds one complete profile roster, revision, and optional
// host choice to a stable digest for CAS retries and owner receipts.
func sessionRosterHash(revision int64, members []uuid.UUID, host *uuid.UUID) ([]byte, error) {
	profiles := make([]string, len(members))
	for i, member := range members {
		profiles[i] = member.String()
	}
	var hostID *string
	if host != nil {
		value := host.String()
		hostID = &value
	}
	body, err := json.Marshal(struct {
		Revision int64    `json:"revision"`
		Members  []string `json:"members"`
		Host     *string  `json:"host_profile_id"`
	}{Revision: revision, Members: profiles, Host: hostID})
	if err != nil {
		return nil, err
	}
	body, err = jsoncanonicalizer.Transform(body)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(body)
	return digest[:], nil
}

func sessionRosterMutationHash(p SessionPrincipal, sessionID uuid.UUID, input SessionRosterUpdateInput) ([]byte, error) {
	if err := validateRosterUpdate(input); err != nil {
		return nil, err
	}
	body, err := json.Marshal(struct {
		APIVersion    string                   `json:"api_version"`
		ApplicationID string                   `json:"application_id"`
		EnvironmentID string                   `json:"environment_id"`
		SessionID     string                   `json:"session_id"`
		Request       SessionRosterUpdateInput `json:"request"`
	}{
		APIVersion: "v1", ApplicationID: p.ApplicationID.String(),
		EnvironmentID: p.EnvironmentID.String(), SessionID: sessionID.String(), Request: input,
	})
	if err != nil {
		return nil, err
	}
	body, err = jsoncanonicalizer.Transform(body)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(body)
	return digest[:], nil
}
