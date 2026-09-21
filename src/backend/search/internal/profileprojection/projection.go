// Package profileprojection validates User-owned profile projection events
// before a Search inbox may persist or apply them.
package profileprojection

import (
	"errors"
	"strings"

	"voice/backend/pkg/searchnormalization"
)

const protocolVersion1 = 1

type Kind uint8

const (
	Upsert Kind = iota + 1
	Delete
)

// Event is the Search-side representation of the frozen v1 User event.
// The transport adapter must populate it only from the authoritative envelope.
type Event struct {
	ProtocolVersion      int
	EventID              string
	ProfileID            string
	SourceRevision       uint64
	Kind                 Kind
	Username             string
	DisplayName          string
	UsernameSearchKey    string
	DisplayNameSearchKey string
	NormalizationVersion int
}

// State is the durable per-profile conditional-apply record. The persistence
// adapter must update it and the corresponding projection in one transaction.
type State struct {
	ProfileID      string
	SourceRevision uint64
	PayloadHash    string
	Tombstoned     bool
}

type ApplyResult uint8

const (
	Applied ApplyResult = iota + 1
	NoopStale
	NoopDuplicate
	Quarantined
)

// ValidateEvent rejects malformed events before they can enter Search's inbox.
// Search revalidates the supplied normalized values; it never substitutes a
// lower(raw) fallback for absent or unsupported keys.
func ValidateEvent(event Event) error {
	if event.ProtocolVersion != protocolVersion1 || strings.TrimSpace(event.EventID) == "" || strings.TrimSpace(event.ProfileID) == "" || event.SourceRevision == 0 {
		return errors.New("invalid profile projection event")
	}
	if event.Kind == Delete {
		if event.Username != "" || event.DisplayName != "" || event.UsernameSearchKey != "" || event.DisplayNameSearchKey != "" || event.NormalizationVersion != 0 {
			return errors.New("delete event must not contain profile fields")
		}
		return nil
	}
	if event.Kind != Upsert || event.NormalizationVersion != searchnormalization.Version1 || event.UsernameSearchKey == "" || event.DisplayNameSearchKey == "" {
		return errors.New("unsupported or missing normalized profile keys")
	}
	if event.UsernameSearchKey != searchnormalization.V1.Normalize(event.Username) || event.DisplayNameSearchKey != searchnormalization.V1.Normalize(event.DisplayName) {
		return errors.New("normalized profile keys do not match source fields")
	}
	return nil
}

// Apply enforces the frozen per-profile revision semantics before persistence.
// Callers persist the returned state with their inbox and projection mutation.
func Apply(state *State, event Event, payloadHash string) (ApplyResult, error) {
	if state == nil {
		return Quarantined, errors.New("projection state is required")
	}
	if err := ValidateEvent(event); err != nil {
		return Quarantined, err
	}
	if strings.TrimSpace(payloadHash) == "" {
		return Quarantined, errors.New("payload hash is required")
	}
	if state.ProfileID != "" && state.ProfileID != event.ProfileID {
		return Quarantined, errors.New("profile projection state mismatch")
	}
	if event.SourceRevision < state.SourceRevision {
		return NoopStale, nil
	}
	if event.SourceRevision == state.SourceRevision && state.SourceRevision != 0 {
		if state.PayloadHash == payloadHash {
			return NoopDuplicate, nil
		}
		return Quarantined, errors.New("conflicting profile projection revision")
	}
	state.ProfileID = event.ProfileID
	state.SourceRevision = event.SourceRevision
	state.PayloadHash = payloadHash
	state.Tombstoned = event.Kind == Delete
	return Applied, nil
}
