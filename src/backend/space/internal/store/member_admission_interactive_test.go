package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMemberAdmissionInteractiveReferenceChangesRequestFingerprint(t *testing.T) {
	const legacy = `{"OperationID":"10000000-0000-0000-0000-000000000001","SpaceID":"20000000-0000-0000-0000-000000000002","ProfileID":"30000000-0000-0000-0000-000000000003","AccountID":"40000000-0000-0000-0000-000000000004","InviteID":null,"Guest":false}`
	var in MemberAdmissionInput
	require.NoError(t, json.Unmarshal([]byte(legacy), &in))
	raw, err := json.Marshal(in)
	require.NoError(t, err)
	require.Equal(t, legacy, string(raw), "nil interactive fields must preserve old request bytes")
	legacyHash := admissionRequestHash(in)
	const interactive = `{"OperationID":"10000000-0000-0000-0000-000000000001","SpaceID":"20000000-0000-0000-0000-000000000002","ProfileID":"30000000-0000-0000-0000-000000000003","AccountID":"40000000-0000-0000-0000-000000000004","InviteID":null,"Guest":false,"JoinAttempt":{"AttemptID":"50000000-0000-0000-0000-000000000005","EvidenceSHA256":"AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="},"DecisionBasisSHA256":"AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI="}`
	require.NoError(t, json.Unmarshal([]byte(interactive), &in))
	require.NotEqual(t, legacyHash, admissionRequestHash(in), "one P must bind exact A, unchanged35 evidence, and separate36 basis")
}

func TestMemberAdmissionInteractivePairAndContext(t *testing.T) {
	in := MemberAdmissionInput{OperationID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), JoinAttempt: &JoinAttemptReference{AttemptID: uuid.New(), EvidenceSHA256: bytes.Repeat([]byte{1}, 32)}, DecisionBasisSHA256: bytes.Repeat([]byte{2}, 32)}
	require.NoError(t, validateMemberAdmissionReference(in))
	i := &MemberAdmissionIntent{Input: in, Generation: 7, BindingSHA256: bytes.Repeat([]byte{3}, 32)}
	digest, err := i.FirstPublicationContextSHA256()
	require.NoError(t, err)
	require.Equal(t, "5e7606c5f50cf14020cda73001cfb66bbed7f5af6adce5c506cba83df908d496", fmt.Sprintf("%x", digest))
	copy := cloneMemberAdmissionInput(in)
	in.JoinAttempt.EvidenceSHA256[0] ^= 1
	in.DecisionBasisSHA256[0] ^= 1
	require.Equal(t, byte(1), copy.JoinAttempt.EvidenceSHA256[0])
	require.Equal(t, byte(2), copy.DecisionBasisSHA256[0])
	for _, invalid := range []MemberAdmissionInput{
		{DecisionBasisSHA256: bytes.Repeat([]byte{2}, 32)},
		{JoinAttempt: copy.JoinAttempt},
		{JoinAttempt: &JoinAttemptReference{EvidenceSHA256: copy.JoinAttempt.EvidenceSHA256}, DecisionBasisSHA256: copy.DecisionBasisSHA256},
		{JoinAttempt: copy.JoinAttempt, DecisionBasisSHA256: make([]byte, 32)},
		{JoinAttempt: copy.JoinAttempt, DecisionBasisSHA256: []byte{2}},
	} {
		require.ErrorIs(t, validateMemberAdmissionReference(invalid), ErrMemberAdmissionConflict)
	}
	for _, generation := range []uint64{0, 8} {
		i.Generation = generation
		changed, err := i.FirstPublicationContextSHA256()
		if generation == 0 {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.NotEqual(t, digest, changed)
		}
	}
}

func TestMemberAdmissionInteractiveMissingBasisProducerIsUnavailable(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "No basis producer", "", "public")
	require.NoError(t, err)
	applyMemberAuthorityIfPresent(t, st)
	in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, ProfileID: uuid.New(), AccountID: uuid.New(), JoinAttempt: &JoinAttemptReference{AttemptID: uuid.New(), EvidenceSHA256: bytes.Repeat([]byte{1}, 32)}, DecisionBasisSHA256: bytes.Repeat([]byte{2}, 32)}
	_, err = st.PrepareMemberAdmission(ctx, in)
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable, "missing scoped36 verifier cannot become noninteractive or caller-asserted success")
	var intents, heads, events, members int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM space_member_admission_intents WHERE operation_id=$1),(SELECT count(*) FROM space_ordinary_member_heads WHERE space_id=$2 AND profile_id=$3),(SELECT count(*) FROM space_member_admission_outbox WHERE operation_id=$1),(SELECT count(*) FROM space_members WHERE space_id=$2 AND profile_id=$3)`, in.OperationID, in.SpaceID, in.ProfileID).Scan(&intents, &heads, &events, &members))
	require.Zero(t, intents)
	require.Zero(t, heads)
	require.Zero(t, events)
	require.Zero(t, members)
}

func TestMemberAdmissionRetiredAttemptRequiresExactNativeBindingAndFloor(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Retired A pin", "", "public")
	require.NoError(t, err)
	applyMemberAuthorityIfPresent(t, st)
	in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, ProfileID: uuid.New(), AccountID: uuid.New(), JoinAttempt: &JoinAttemptReference{AttemptID: uuid.New(), EvidenceSHA256: bytes.Repeat([]byte{1}, 32)}, DecisionBasisSHA256: bytes.Repeat([]byte{2}, 32)}
	request := admissionRequestHash(in)
	binding := bytes.Repeat([]byte{3}, 32)
	// Native SQL fixture only: models the inert output of a successful scoped
	// journal verifier, without claiming a real36 decision or owner observation.
	_, err = pool.Exec(ctx, `INSERT INTO space_ordinary_member_heads(space_id,profile_id,generation,publication_id,state,account_id,admission_binding_sha256) VALUES($1,$2,1,$3,'PREPARING',$4,$5)`, in.SpaceID, in.ProfileID, in.OperationID, in.AccountID, binding)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO space_member_admission_intents(`+memberAdmissionColumns+`) VALUES($1,$2,$3,$4,NULL,false,1,$5,$6,1,'PREPARING',$7,$8,$9)`, in.OperationID, in.SpaceID, in.ProfileID, in.AccountID, request[:], binding, in.JoinAttempt.AttemptID, in.JoinAttempt.EvidenceSHA256, in.DecisionBasisSHA256)
	require.NoError(t, err)
	saved, err := st.LookupMemberAdmission(ctx, in.OperationID)
	require.NoError(t, err)
	require.Equal(t, in, saved.Input)
	require.ErrorIs(t, st.AssertJoinAttemptPublicationRetired(ctx, *in.JoinAttempt, in.OperationID), ErrJoinAttemptUnavailable, "outer unscoped calls are never a retirement assertion")
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer rollbackLifecycleTx(ctx, tx)
	require.NoError(t, lockLifecycleSpace(ctx, tx, in.SpaceID))
	scoped := *st
	scoped.tx = tx
	require.ErrorIs(t, scoped.AssertJoinAttemptPublicationRetired(ctx, *in.JoinAttempt, in.OperationID), ErrJoinAttemptUnavailable, "PREPARING is not retired")
	_, err = tx.Exec(ctx, `UPDATE space_member_admission_intents SET state='RETIRED' WHERE operation_id=$1`, in.OperationID)
	require.NoError(t, err)
	require.ErrorIs(t, scoped.AssertJoinAttemptPublicationRetired(ctx, *in.JoinAttempt, in.OperationID), ErrJoinAttemptUnavailable, "intent state alone cannot substitute permanent floor")
	_, err = tx.Exec(ctx, `UPDATE space_ordinary_member_heads SET state='RETIRED',generation=2,retired_through=2 WHERE space_id=$1 AND profile_id=$2`, in.SpaceID, in.ProfileID)
	require.NoError(t, err)
	require.NoError(t, scoped.AssertJoinAttemptPublicationRetired(ctx, *in.JoinAttempt, in.OperationID))
	_, err = tx.Exec(ctx, `UPDATE space_member_admission_intents SET request_sha256=$2 WHERE operation_id=$1`, in.OperationID, bytes.Repeat([]byte{9}, 32))
	require.NoError(t, err)
	require.ErrorIs(t, scoped.AssertJoinAttemptPublicationRetired(ctx, *in.JoinAttempt, in.OperationID), ErrJoinAttemptUnavailable, "corrupt native request binding is not exact retirement evidence")
	_, err = tx.Exec(ctx, `UPDATE space_member_admission_intents SET request_sha256=$2 WHERE operation_id=$1`, in.OperationID, request[:])
	require.NoError(t, err)
	wrong := *in.JoinAttempt
	wrong.AttemptID = uuid.New()
	require.ErrorIs(t, scoped.AssertJoinAttemptPublicationRetired(ctx, wrong, in.OperationID), ErrJoinAttemptUnavailable)
	wrong = *in.JoinAttempt
	wrong.EvidenceSHA256 = bytes.Repeat([]byte{4}, 32)
	require.ErrorIs(t, scoped.AssertJoinAttemptPublicationRetired(ctx, wrong, in.OperationID), ErrJoinAttemptUnavailable)
	require.ErrorIs(t, scoped.AssertJoinAttemptPublicationRetired(ctx, *in.JoinAttempt, uuid.New()), ErrJoinAttemptUnavailable)
	require.NoError(t, tx.Rollback(ctx))
	saved, err = st.LookupMemberAdmission(ctx, in.OperationID)
	require.NoError(t, err)
	require.Equal(t, "PREPARING", saved.State, "retirement and floor fixture changes roll back together")
}
