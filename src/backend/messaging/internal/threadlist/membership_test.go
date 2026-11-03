package threadlist

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func membershipFixture() MembershipRecord {
	e := MembershipRecord{EventID: uuid.New(), ChatID: uuid.New(), ProfileID: uuid.New(),
		Incarnation: uuid.New(), Offset: 1, Revision: 1, Lineage: uuid.New(), Kind: MemberJoined}
	e.Sources = []MembershipSource{
		{Owner: "chat", Scope: e.ChatID, Incarnation: e.Incarnation, Revision: 1},
		{Owner: "user", Scope: e.ProfileID, Incarnation: uuid.New(), Revision: 1},
	}
	return e
}

func TestMembershipRecordValidation(t *testing.T) {
	good := membershipFixture()
	require.NoError(t, good.Validate())
	for _, change := range []struct {
		name string
		edit func(*MembershipRecord)
	}{
		{"event", func(e *MembershipRecord) { e.EventID = uuid.Nil }},
		{"chat", func(e *MembershipRecord) { e.ChatID = uuid.Nil }},
		{"profile", func(e *MembershipRecord) { e.ProfileID = uuid.Nil }},
		{"incarnation", func(e *MembershipRecord) { e.Incarnation = uuid.Nil }},
		{"offset", func(e *MembershipRecord) { e.Offset = 0 }},
		{"revision", func(e *MembershipRecord) { e.Revision = -1 }},
		{"lineage", func(e *MembershipRecord) { e.Lineage = uuid.Nil }},
		{"kind", func(e *MembershipRecord) { e.Kind = "allow" }},
		{"sources_missing", func(e *MembershipRecord) { e.Sources = nil }},
		{"sources_duplicate", func(e *MembershipRecord) {
			e.Sources = append(append([]MembershipSource{}, e.Sources...), e.Sources[0])
		}},
		{"source_unknown", func(e *MembershipRecord) {
			e.Sources = append([]MembershipSource{}, e.Sources...)
			e.Sources[0].Owner = "gateway"
		}},
		{"source_revision", func(e *MembershipRecord) {
			e.Sources = append([]MembershipSource{}, e.Sources...)
			e.Sources[0].Revision = -1
		}},
		{"source_revocation", func(e *MembershipRecord) {
			e.Sources = append([]MembershipSource{}, e.Sources...)
			e.Sources[0].RevocationRevision = 2
		}},
		{"source_scope", func(e *MembershipRecord) {
			e.Sources = append([]MembershipSource{}, e.Sources...)
			e.Sources[0].Scope = uuid.New()
		}},
		{"source_incarnation", func(e *MembershipRecord) {
			e.Sources = append([]MembershipSource{}, e.Sources...)
			e.Sources[0].Incarnation = uuid.Nil
		}},
	} {
		t.Run(change.name, func(t *testing.T) {
			e := good
			change.edit(&e)
			require.ErrorIs(t, e.Validate(), ErrInvalidMembership)
		})
	}
}

func TestMembershipPairRejectsStaleAndRetiredLineage(t *testing.T) {
	e := membershipFixture()
	pair, err := (MembershipPair{}).Next(e)
	require.NoError(t, err)
	require.Equal(t, MemberBuilding, pair.Status)
	require.Equal(t, e.Lineage, pair.Lineage)
	require.Equal(t, e.Revision, pair.Revision)
	_, err = pair.Next(e)
	require.ErrorIs(t, err, ErrMembershipConflict, "inbox handles exact duplicates before reduction")
	e.EventID, e.Offset, e.Revision, e.Kind = uuid.New(), 2, 2, MemberRemoved
	removed, err := pair.Next(e)
	require.NoError(t, err)
	require.Equal(t, MemberRevoked, removed.Status)
	e.EventID, e.Offset, e.Revision, e.Kind = uuid.New(), 3, 3, MemberJoined
	_, err = removed.Next(e)
	require.ErrorIs(t, err, ErrMembershipConflict, "a revoke/rejoin cannot revive the old cursor lineage")
	e.Lineage = uuid.New()
	rejoined, err := removed.Next(e)
	require.NoError(t, err)
	require.Equal(t, MemberBuilding, rejoined.Status)
	// A late removal of the old lineage cannot revoke the new one.
	e.EventID, e.Offset, e.Revision, e.Kind, e.Lineage = uuid.New(), 4, 4, MemberLeft, removed.Lineage
	_, err = rejoined.Next(e)
	require.ErrorIs(t, err, ErrMembershipConflict)
}

func TestMembershipPairRequiresNewLineageOnlyAfterRevocation(t *testing.T) {
	e := membershipFixture()
	pair, err := (MembershipPair{}).Next(e)
	require.NoError(t, err)
	e.EventID, e.Offset, e.Revision = uuid.New(), 2, 2
	_, err = pair.Next(e)
	require.ErrorIs(t, err, ErrMembershipConflict, "no-op refresh is a barrier, not another join")
	e.Kind = MemberBarrier
	fenced, err := pair.Next(e)
	require.NoError(t, err)
	require.Equal(t, MemberRevoked, fenced.Status, "uncertain restriction retires the old lineage")
}
