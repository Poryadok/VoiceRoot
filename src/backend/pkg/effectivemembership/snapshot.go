// Package effectivemembership defines the private complete-snapshot intake shared
// by Chat's generation exporter and Messaging's baseline builder. It grants no
// read permission and does not replace fresh point admission.
package effectivemembership

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrSnapshotUnavailable = errors.New("effective membership snapshot unavailable")

const MaxPageMembers = 512

type Source struct {
	Owner                        string
	Scope, Incarnation           uuid.UUID
	Revision, RevocationRevision int64
}

type Member struct {
	ProfileID uuid.UUID
	Revision  int64
	Lineage   uuid.UUID
	Allowed   bool
	Sources   []Source
}

// Export is the immutable owner seal. Count and Digest cover the complete
// generation, not a page or a filtered list. Heads includes every source scope
// validated by Chat for this generation, including negative eligibility facts.
type Export struct {
	ProtocolVersion                                    uint32
	ChatID, Incarnation, SnapshotID, Generation, PinID uuid.UUID
	Q                                                  int64
	Count                                              uint64
	Digest                                             [32]byte
	RetainUntil                                        time.Time
	Heads                                              []Source
}

type Page struct {
	ChatID, Incarnation, SnapshotID, Generation, PinID uuid.UUID
	Q                                                  int64
	Position                                           uint64
	Cursor, NextCursor                                 string
	Terminal                                           bool
	Members                                            []Member
}

// SourceReader is implemented by the authenticated owner adapter, never an
// ordinary viewer-filtered roster. Begin registers retention before export;
// Validate attests the CURRENT Chat journal incarnation, active original pin,
// and unchanged immutable G/Q seal after all pages have been read. Heads bind
// G's validated historical cut; they are not a live permission grant. The adapter
// must bind caller, exact RPC/request, scope, handle and continuation over TLS.
type SourceReader interface {
	Begin(context.Context, uuid.UUID) (Export, error)
	Page(context.Context, Export, string, uint64) (Page, error)
	Validate(context.Context, Export) (Export, error)
}

// ValidatedSnapshot can only be produced by successful complete intake. Its
// private fields prevent a request's claimed terminal flag/hash from becoming
// an installation capability without calling the authenticated owner adapter.
type ValidatedSnapshot struct {
	export  Export
	members []Member
}

func (s ValidatedSnapshot) Export() Export    { return cloneExport(s.export) }
func (s ValidatedSnapshot) Members() []Member { return cloneMembers(s.members) }

func Receive(ctx context.Context, source SourceReader, chat uuid.UUID) (ValidatedSnapshot, error) {
	if ctx == nil || source == nil || chat == uuid.Nil {
		return ValidatedSnapshot{}, ErrSnapshotUnavailable
	}
	if err := ctx.Err(); err != nil {
		return ValidatedSnapshot{}, errors.Join(ErrSnapshotUnavailable, err)
	}
	h, err := source.Begin(ctx, chat)
	if err != nil {
		return ValidatedSnapshot{}, errors.Join(ErrSnapshotUnavailable, err)
	}
	h = cloneExport(h)
	if h.ChatID != chat || !validExport(h) {
		return ValidatedSnapshot{}, ErrSnapshotUnavailable
	}
	binding := ExportDigest(h)
	heads := indexMemberHeads(h)
	members := make([]Member, 0, min(h.Count, MaxPageMembers))
	seen := make(map[uuid.UUID]bool)
	cursors := map[string]bool{"": true}
	cursor := ""
	var position uint64
	for {
		if err = ctx.Err(); err != nil {
			return ValidatedSnapshot{}, errors.Join(ErrSnapshotUnavailable, err)
		}
		p, pageErr := source.Page(ctx, cloneExport(h), cursor, position)
		if pageErr != nil {
			return ValidatedSnapshot{}, errors.Join(ErrSnapshotUnavailable, pageErr)
		}
		if p.ChatID != h.ChatID || p.Incarnation != h.Incarnation || p.SnapshotID != h.SnapshotID ||
			p.Generation != h.Generation || p.PinID != h.PinID || p.Q != h.Q || p.Position != position || p.Cursor != cursor ||
			len(p.Members) > MaxPageMembers || uint64(len(p.Members)) > h.Count-position ||
			p.Terminal != (p.NextCursor == "") || (!p.Terminal && len(p.Members) == 0) {
			return ValidatedSnapshot{}, ErrSnapshotUnavailable
		}
		for _, member := range p.Members {
			if seen[member.ProfileID] || validateMember(h.ChatID, heads, member) != nil {
				return ValidatedSnapshot{}, ErrSnapshotUnavailable
			}
			seen[member.ProfileID] = true
		}
		members = append(members, cloneMembers(p.Members)...)
		position += uint64(len(p.Members))
		if p.Terminal {
			if position != h.Count || ContentDigest(h, members) != h.Digest {
				return ValidatedSnapshot{}, ErrSnapshotUnavailable
			}
			break
		}
		if cursors[p.NextCursor] {
			return ValidatedSnapshot{}, ErrSnapshotUnavailable
		}
		cursors[p.NextCursor] = true
		cursor = p.NextCursor
	}
	// All page reads finish before the owner attests its current incarnation and
	// original pin/seal. A static/cached header is not an implementation of
	// Validate. The builder separately obtains fresh admission after replay.
	final, err := source.Validate(ctx, cloneExport(h))
	if err != nil {
		return ValidatedSnapshot{}, errors.Join(ErrSnapshotUnavailable, err)
	}
	if !validExport(final) || ExportDigest(final) != binding {
		return ValidatedSnapshot{}, ErrSnapshotUnavailable
	}
	if err = ctx.Err(); err != nil {
		return ValidatedSnapshot{}, errors.Join(ErrSnapshotUnavailable, err)
	}
	return ValidatedSnapshot{export: h, members: members}, nil
}

type sourceKey struct {
	owner string
	scope uuid.UUID
}

// ValidateSources checks structural coverage and coordinate identity. The owner
// adapter must independently prove that the enumerated dependencies are complete
// and are validated at one common source cut.
func ValidateSources(chat, viewer uuid.UUID, sources []Source) error {
	seen := make(map[sourceKey]bool)
	chatFound, viewerFound := false, viewer == uuid.Nil
	var space, role uuid.UUID
	for _, s := range sources {
		key := sourceKey{s.Owner, s.Scope}
		if s.Scope == uuid.Nil || s.Incarnation == uuid.Nil || s.Revision < 0 || s.RevocationRevision < 0 ||
			s.RevocationRevision > s.Revision || seen[key] {
			return ErrSnapshotUnavailable
		}
		seen[key] = true
		switch s.Owner {
		case "chat":
			if s.Scope != chat {
				return ErrSnapshotUnavailable
			}
			chatFound = true
		case "user":
			viewerFound = viewerFound || s.Scope == viewer
		case "space":
			if space != uuid.Nil {
				return ErrSnapshotUnavailable
			}
			space = s.Scope
		case "role":
			if role != uuid.Nil {
				return ErrSnapshotUnavailable
			}
			role = s.Scope
		default:
			return ErrSnapshotUnavailable
		}
	}
	if chat == uuid.Nil || !chatFound || !viewerFound || space != role {
		return ErrSnapshotUnavailable
	}
	return nil
}

func validExport(h Export) bool {
	year := h.RetainUntil.Year()
	return h.ProtocolVersion == 1 && h.ChatID != uuid.Nil && h.Incarnation != uuid.Nil && h.SnapshotID != uuid.Nil &&
		h.Generation != uuid.Nil && h.PinID != uuid.Nil && h.Q >= 0 && h.Digest != ([32]byte{}) &&
		!h.RetainUntil.IsZero() && year >= 0 && year <= 9999 && time.Now().Before(h.RetainUntil) && ValidateSources(h.ChatID, uuid.Nil, h.Heads) == nil
}

func ValidateMember(h Export, member Member) error {
	return validateMember(h.ChatID, indexMemberHeads(h), member)
}

type memberHeads struct {
	all    map[sourceKey]Source
	common []sourceKey
}

func indexMemberHeads(h Export) memberHeads {
	index := memberHeads{all: make(map[sourceKey]Source, len(h.Heads))}
	for _, head := range h.Heads {
		index.all[sourceKey{head.Owner, head.Scope}] = head
	}
	for key := range index.all {
		if key.owner != "user" {
			index.common = append(index.common, key)
		}
	}
	return index
}

func validateMember(chat uuid.UUID, heads memberHeads, member Member) error {
	if member.ProfileID == uuid.Nil || member.Revision < 0 || (member.Allowed && (member.Revision == 0 || member.Lineage == uuid.Nil)) ||
		ValidateSources(chat, member.ProfileID, member.Sources) != nil {
		return ErrSnapshotUnavailable
	}
	covered := make(map[sourceKey]bool, len(member.Sources))
	for _, s := range member.Sources {
		key := sourceKey{s.Owner, s.Scope}
		if head, ok := heads.all[key]; !ok || head != s || (s.Owner == "user" && s.Scope != member.ProfileID) {
			return ErrSnapshotUnavailable
		}
		covered[key] = true
	}
	for _, key := range heads.common {
		if !covered[key] {
			return ErrSnapshotUnavailable
		}
	}
	return nil
}
