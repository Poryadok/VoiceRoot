package threadlist

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"

	"github.com/google/uuid"
)

var (
	ErrInvalidMembership  = errors.New("invalid thread membership record")
	ErrMembershipConflict = errors.New("thread membership conflict")
)

const (
	MemberJoined   = "joined"
	MemberRemoved  = "removed"
	MemberLeft     = "left"
	MemberBarrier  = "barrier"
	MemberBuilding = "BUILDING"
	MemberRevoked  = "REVOKED"
)

type MembershipRecord struct {
	EventID, ChatID, ProfileID, Incarnation, Lineage uuid.UUID
	Offset, Revision                                 int64
	Kind                                             string
	Sources                                          []MembershipSource
}

// MembershipSource binds one authoritative coordinate in Chat's complete cut.
// The bridge proves coverage; the inbox independently checks coordinate identity.
type MembershipSource struct {
	Owner                        string
	Scope, Incarnation           uuid.UUID
	Revision, RevocationRevision int64
}

func (e MembershipRecord) Validate() error {
	if e.EventID == uuid.Nil || e.ChatID == uuid.Nil || e.ProfileID == uuid.Nil ||
		e.Incarnation == uuid.Nil || e.Lineage == uuid.Nil || e.Offset < 1 || e.Revision < 1 {
		return ErrInvalidMembership
	}
	seen := make(map[struct {
		owner string
		scope uuid.UUID
	}]bool)
	chat, viewer := false, false
	var space, role uuid.UUID
	for _, source := range e.Sources {
		key := struct {
			owner string
			scope uuid.UUID
		}{source.Owner, source.Scope}
		if source.Scope == uuid.Nil || source.Incarnation == uuid.Nil || source.Revision < 0 ||
			source.RevocationRevision < 0 || source.RevocationRevision > source.Revision || seen[key] {
			return ErrInvalidMembership
		}
		seen[key] = true
		switch source.Owner {
		case "chat":
			if source.Scope != e.ChatID {
				return ErrInvalidMembership
			}
			chat = true
		case "user":
			viewer = viewer || source.Scope == e.ProfileID
		case "space":
			if space != uuid.Nil {
				return ErrInvalidMembership
			}
			space = source.Scope
		case "role":
			if role != uuid.Nil {
				return ErrInvalidMembership
			}
			role = source.Scope
		default:
			return ErrInvalidMembership
		}
	}
	if !chat || !viewer || space != role {
		return ErrInvalidMembership
	}
	switch e.Kind {
	case MemberJoined, MemberRemoved, MemberLeft, MemberBarrier:
		return nil
	default:
		return ErrInvalidMembership
	}
}

func canonicalMembership(e MembershipRecord) ([]byte, error) {
	// Coordinates are a set. Normalize a private copy so equivalent transport
	// ordering is idempotent without mutating the caller's signed source data.
	e.Sources = append([]MembershipSource(nil), e.Sources...)
	sort.Slice(e.Sources, func(i, j int) bool {
		a, b := e.Sources[i], e.Sources[j]
		if a.Owner != b.Owner {
			return a.Owner < b.Owner
		}
		return bytes.Compare(a.Scope[:], b.Scope[:]) < 0
	})
	return json.Marshal(e)
}

type MembershipPair struct {
	Revision int64
	Lineage  uuid.UUID
	Status   string
}

func (p MembershipPair) Next(e MembershipRecord) (MembershipPair, error) {
	if err := e.Validate(); err != nil {
		return MembershipPair{}, err
	}
	if e.Revision <= p.Revision {
		return MembershipPair{}, ErrMembershipConflict
	}
	next := MembershipPair{Revision: e.Revision, Lineage: e.Lineage, Status: MemberRevoked}
	if e.Kind == MemberJoined {
		if p.Revision > 0 && (p.Status != MemberRevoked || p.Lineage == e.Lineage) {
			return MembershipPair{}, ErrMembershipConflict
		}
		next.Status = MemberBuilding
	} else if p.Revision > 0 && p.Lineage != e.Lineage {
		return MembershipPair{}, ErrMembershipConflict
	}
	return next, nil
}
