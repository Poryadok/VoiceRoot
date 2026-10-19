package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	authv1 "voice.app/voice/auth/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/memberadmission"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/admissionauthority"
	"voice/backend/space/internal/entrypolicy"
	"voice/backend/space/internal/memberadmissionrole"
)

type aliasRoleProtocolFixture func(context.Context, *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error)

func (f aliasRoleProtocolFixture) EnsureDefaultMemberRole(ctx context.Context, r *rolev1.EnsureDefaultMemberRoleRequest, _ ...grpc.CallOption) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
	return f(ctx, r)
}

// Genuine native45 serving membership with retained READY history. Controlled
// owner/Role clients prove the alias protocol, not actual Java/Role assembly or
// a native FIRST publication receipt.
func TestJoinApprovalAliasNative45ObservationalJournalAndCurrentPermission(t *testing.T) {
	for _, family := range []string{"v1", "legacy_v2"} {
		t.Run(family, func(t *testing.T) { testJoinApprovalAliasNative45(t, family) })
	}
}

func testJoinApprovalAliasNative45(t *testing.T, family string) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "41")
	admin := &SpaceStore{Pool: db.Pool}
	create, account, profile := signedNativeCreatorCurrent(t, admin, &spacev1.CreateSpaceRequest{Name: "Alias reviewer native45", Visibility: "public"})
	space, err := create()
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	db.Run(true, "up")
	_, err = admin.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	st := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	_, err = st.SetEntryPolicy(ctx, SetEntryPolicyInput{SpaceID: space.ID, ActorProfileID: profile, OperationID: uuid.New(), ExpectedVersion: 1, Policy: entrypolicy.Policy{SchemaVersion: 1, RequireManualApproval: true, Questions: []entrypolicy.Question{}}})
	require.NoError(t, err)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	var q *JoinRequestGroup
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(local *SpaceStore) error { var e error; q, e = local.AttachJoinRequestAttempt(ctx, a.ID); return e }))
	var parent JoinDecisionBasis
	var legacyParent JoinLegacyDecisionBasis
	var parentSHA []byte
	if family == "v1" {
		parent, parentSHA = seedJoinDecisionHistory(t, st, in, a)
	} else {
		var binding, submission []byte
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT binding_sha256,submission_sha256 FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&binding, &submission))
		legacyRequest := &spacev1.JoinSpaceRequest{SpaceId: space.ID.String()}
		wire, e := proto.MarshalOptions{Deterministic: true}.Marshal(legacyRequest)
		require.NoError(t, e)
		hash := sha256.Sum256(wire)
		legacyParent = JoinLegacyDecisionBasis{Context: JoinLegacyDecisionContext{Kind: JoinLegacyPending, OperationOrigin: JoinOperationServerNoKey, SpaceID: space.ID, AttemptID: a.ID, AttemptBindingSHA256: binding, SubmissionSHA256: submission, ActorAccountID: in.AccountID, ActorProfileID: in.ProfileID, OperationID: uuid.New(), RequestMethod: spacev1.SpaceService_JoinSpace_FullMethodName, RequestSHA256: hash[:]}, ApplicantObservationSHA256: bytes.Repeat([]byte{2}, 32)}
		parentSHA = seedLegacyJoinDecisionHistory(t, st, legacyParent)
	}
	selector := a.ID.String()
	originalRequest := &spacev1.ApproveJoinRequestRequest{SpaceId: space.ID.String(), RequestId: q.ID.String(), OperationId: uuid.NewString(), AttemptId: &selector}
	original, _ := signedApprovalOperationFixture(t, originalRequest, account, profile)
	_, err = st.ResolveJoinRequestApproval(ctx, original.input, q.ID, &a.ID)
	require.NoError(t, err)
	approval := JoinAttemptApprovalInput{SpaceID: space.ID, AttemptID: a.ID, ActorAccountID: account, ActorProfileID: profile, OperationID: original.input.OperationID}
	_, err = st.ReserveJoinAttemptApproval(ctx, approval)
	require.NoError(t, err)
	a, err = st.ApproveJoinAttempt(ctx, approval, func(context.Context, *SpaceStore, JoinAttemptApprovalInput) error { return nil })
	require.NoError(t, err, "retained READY fixture, not a public approval allow")
	var basisSHA [32]byte
	var raw []byte
	version := 1
	if family == "v1" {
		basis := JoinDecisionBasis{Context: parent.Context, ApplicantObservationSHA256: bytes.Repeat([]byte{3}, 32), ReviewerObservationSHA256: bytes.Repeat([]byte{4}, 32)}
		c := &basis.Context
		c.Kind = JoinDecisionApproveReady
		c.ActorAccountID = account
		c.ActorProfileID = profile
		c.OperationID = original.input.OperationID
		c.RequestMethod = JoinOperationApprove
		c.RequestSHA256 = original.input.RequestSHA256
		c.ReadyEvidenceSHA256 = a.EvidenceSHA256
		c.SubmitBasisSHA256 = parentSHA
		basisSHA, err = basis.SHA256()
		require.NoError(t, err)
		raw, err = json.Marshal(basis)
		require.NoError(t, err)
	} else {
		version = 2
		basis := JoinLegacyDecisionBasis{Context: legacyParent.Context, ApplicantObservationSHA256: bytes.Repeat([]byte{3}, 32), ReviewerObservationSHA256: bytes.Repeat([]byte{4}, 32)}
		c := &basis.Context
		c.Kind = JoinApproveLegacyReady
		c.OperationOrigin = JoinOperationClientKey
		c.ActorAccountID = account
		c.ActorProfileID = profile
		c.OperationID = original.input.OperationID
		c.RequestMethod = JoinOperationApprove
		c.RequestSHA256 = original.input.RequestSHA256
		c.ReadyEvidenceSHA256 = a.EvidenceSHA256
		c.PendingBasisSHA256 = parentSHA
		basisSHA, err = basis.SHA256()
		require.NoError(t, err)
		raw, err = json.Marshal(basis)
		require.NoError(t, err)
	}
	_, err = st.Pool.Exec(ctx, `UPDATE space_join_operations SET state='DECIDED',basis_version=$5,operation_origin=1,decision_basis=$3,decision_basis_sha256=$4 WHERE actor_profile_id=$1 AND operation_id=$2`, profile, original.input.OperationID, raw, basisSHA[:], version)
	require.NoError(t, err)
	aliasRequest := proto.Clone(originalRequest).(*spacev1.ApproveJoinRequestRequest)
	aliasRequest.OperationId = uuid.NewString()
	operation, elapsed := signedApprovalOperationFixture(t, aliasRequest, account, profile)
	p, err := st.PrepareJoinApprovalAlias(ctx, operation)
	require.NoError(t, err)
	require.NotNil(t, p)
	assertJoinAliasOwnerProtocol(t, p, false, in.AccountID, in.ProfileID)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	allow := true
	ownerCalls, permissionCalls := 0, 0
	subjects := &admissionauthority.Client{Issuer: issuer, Auth: native44SettingsSubjectFixture(func(_ context.Context, r *authv1.ObserveSpaceAdmissionSubjectRequest) (*authv1.ObserveSpaceAdmissionSubjectResponse, error) {
		ownerCalls++
		raw, e := proto.MarshalOptions{Deterministic: true}.Marshal(r)
		require.NoError(t, e)
		hash := sha256.Sum256(raw)
		return &authv1.ObserveSpaceAdmissionSubjectResponse{RequestBindingSha256: hash[:], AccountId: r.AccountId, ProfileId: r.ProfileId, AccountType: "regular", AccountSecurityRevision: 1, ProfileRevision: 1, SessionEpochFloor: 7, Decision: authv1.SpaceAdmissionDecision_SPACE_ADMISSION_DECISION_ELIGIBLE}, nil
	})}
	reviewers := &admissionauthority.ReviewerClient{Issuer: issuer, Role: native44SettingsPermissionFixture(func(_ context.Context, r *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
		permissionCalls++
		require.Equal(t, admissionauthority.ReviewJoinPermission, r.PermissionName)
		return &rolev1.CheckPermissionResponse{Allowed: allow}, nil
	})}
	seal := func(p *JoinApprovalAliasProposal) (*VerifiedJoinApprovalAlias, error) {
		a, r, e := p.OwnerRequests()
		if e != nil {
			return nil, e
		}
		app, e := subjects.Observe(ctx, a)
		if e != nil {
			return nil, e
		}
		reviewer, e := subjects.Observe(ctx, r)
		if e != nil {
			return nil, e
		}
		permission, e := reviewers.Observe(ctx, reviewer, r)
		if e != nil {
			return nil, e
		}
		return p.Seal(app, permission)
	}
	// Even a genuine private READ permission cannot stand in for APPROVE.
	appRequest, reviewRequest, err := p.OwnerRequests()
	require.NoError(t, err)
	appProof, err := subjects.Observe(ctx, appRequest)
	require.NoError(t, err)
	readRequest := proto.Clone(reviewRequest).(*authv1.ObserveSpaceAdmissionSubjectRequest)
	readRequest.Phase = authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ
	readSubject, err := subjects.Observe(ctx, readRequest)
	require.NoError(t, err)
	readPermission, err := reviewers.Observe(ctx, readSubject, readRequest)
	require.NoError(t, err)
	_, err = p.Seal(appProof, readPermission)
	require.Error(t, err)
	d, err := seal(p)
	require.NoError(t, err)
	response, recovery, err := st.ReadJoinApprovalAlias(ctx, d)
	require.NoError(t, err)
	require.Nil(t, response)
	require.Equal(t, basisSHA[:], recovery.DecisionBasisSHA256)
	require.Nil(t, recovery.PublicationOperationID)
	var state string
	var ownBasis, observed, outcome []byte
	var publication *uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,decision_basis_sha256,observed_basis_sha256,outcome_proto,publication_operation_id FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, operation.input.OperationID).Scan(&state, &ownBasis, &observed, &outcome, &publication))
	require.Equal(t, "DECIDED", state)
	require.Nil(t, ownBasis)
	require.Equal(t, basisSHA[:], observed)
	require.Nil(t, outcome)
	require.Nil(t, publication)
	// Exact alias retries collect current permission; they never inherit c7.
	p, err = st.PrepareJoinApprovalAlias(ctx, operation)
	require.NoError(t, err)
	allow = false
	previousOwner, previousPermission := ownerCalls, permissionCalls
	_, err = seal(p)
	require.Error(t, err)
	require.Equal(t, previousPermission+1, permissionCalls)
	require.Equal(t, previousOwner+2, ownerCalls)
	// An earlier allow cannot survive the real initiating call's strict bound.
	elapsed.Store(int64(5 * time.Second))
	_, _, err = st.ReadJoinApprovalAlias(ctx, d)
	require.Error(t, err)
	// A new alias's observed-basis CAS must roll back if its initiating call
	// expires while waiting on that exact journal row.
	allow = true
	blockedRequest := proto.Clone(aliasRequest).(*spacev1.ApproveJoinRequestRequest)
	blockedRequest.OperationId = uuid.NewString()
	blockedOperation, blockedElapsed := signedApprovalOperationFixture(t, blockedRequest, account, profile)
	blockedProposal, err := st.PrepareJoinApprovalAlias(ctx, blockedOperation)
	require.NoError(t, err)
	blockedDecision, err := seal(blockedProposal)
	require.NoError(t, err)
	holder, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer holder.Rollback(ctx)
	var pid int32
	require.NoError(t, holder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	var held uuid.UUID
	require.NoError(t, holder.QueryRow(ctx, `SELECT operation_id FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 FOR UPDATE`, profile, blockedOperation.input.OperationID).Scan(&held))
	done := make(chan error, 1)
	go func() { _, _, e := st.ReadJoinApprovalAlias(ctx, blockedDecision); done <- e }()
	require.Eventually(t, func() bool {
		var waiting bool
		return db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&waiting) == nil && waiting
	}, 3*time.Second, 5*time.Millisecond)
	blockedElapsed.Store(int64(5 * time.Second))
	require.NoError(t, holder.Commit(ctx))
	require.Error(t, <-done)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,observed_basis_sha256 FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, blockedOperation.input.OperationID).Scan(&state, &observed))
	require.Equal(t, "RESERVED", state)
	require.Nil(t, observed)
	var attempts, bases, intents int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_attempts WHERE space_id=$1`, space.ID).Scan(&attempts))
	require.Equal(t, 1, attempts)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_join_operations WHERE selected_attempt_id=$1 AND decision_basis_sha256 IS NOT NULL`, a.ID).Scan(&bases))
	require.Equal(t, 2, bases, "original submit and original approval only")
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_member_admission_intents WHERE profile_id=$1`, in.ProfileID).Scan(&intents))
	require.Zero(t, intents, "observing READY never prepares another P")
	// Controlled Role protocol receipt below exercises only local native FIRST
	// atomicity. It is not a real RolePG grant, delegation/default-role proof,
	// remote Auth eligibility, or full public serving-positive qualification.
	ensureCalls := 0
	selectedRole := uuid.New()
	roles := &memberadmissionrole.Client{Issuer: issuer, Role: aliasRoleProtocolFixture(func(_ context.Context, r *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
		ensureCalls++
		binding := memberadmission.EnsureBinding{ProtocolVersion: r.ProtocolVersion, SpaceID: r.SpaceId, ProfileID: r.ProfileId, AdmissionOperationID: r.AdmissionOperationId, MembershipGeneration: r.MembershipGeneration, AdmissionBindingSHA256: r.AdmissionBindingSha256}
		digest, e := memberadmission.EnsureReceiptFingerprint(binding, selectedRole.String())
		require.NoError(t, e)
		return &rolev1.EnsureDefaultMemberRoleResponse{Receipt: &rolev1.DefaultMemberRoleReceipt{ProtocolVersion: r.ProtocolVersion, SpaceId: r.SpaceId, ProfileId: r.ProfileId, AdmissionOperationId: r.AdmissionOperationId, MembershipGeneration: r.MembershipGeneration, AdmissionBindingSha256: bytes.Clone(r.AdmissionBindingSha256), RoleId: selectedRole.String(), ReceiptSha256: digest[:]}}, nil
	})}
	// Same reviewer/new operation is an alias too. A fresh real signed call
	// resumes the original winner, never overwrites its identity or basis.
	continuing, continuingElapsed := signedApprovalOperationFixture(t, aliasRequest, account, profile)
	continuingProposal, err := st.PrepareJoinApprovalAlias(ctx, continuing)
	require.NoError(t, err)
	continuingDecision, err := seal(continuingProposal)
	require.NoError(t, err)
	_, recovery, err = st.ReadJoinApprovalAlias(ctx, continuingDecision)
	require.NoError(t, err)
	intent, err := st.PrepareJoinDecisionAdmission(ctx, recovery.DecisionBasisSHA256)
	require.NoError(t, err)
	hookCalls := 0
	var restrictionErr error
	hook := func(ctx context.Context, local *SpaceStore, native *MemberAdmissionIntent, _ *MembershipRow) error {
		hookCalls++
		restrictionErr = local.MarkJoinApprovalAliasPublishedOutcome(ctx, continuingDecision, native)
		if restrictionErr != nil {
			return ErrJoinAttemptUnavailable
		}
		return nil
	}
	// Hold ORIGINAL acknowledgement SQL. Alias expiry must roll the entire
	// FIRST attempt back without retiring the original PREPARING candidate.
	ackHolder, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer ackHolder.Rollback(ctx)
	require.NoError(t, ackHolder.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	require.NoError(t, ackHolder.QueryRow(ctx, `SELECT operation_id FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2 FOR UPDATE`, profile, original.input.OperationID).Scan(&held))
	done = make(chan error, 1)
	go func() {
		_, e := st.CompleteInteractiveMemberAdmissionWithPublicationCall(ctx, intent.Input.OperationID, roles, subjects, reviewers, nil, hook, continuing.call, continuing.request)
		done <- e
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		return db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&waiting) == nil && waiting
	}, 3*time.Second, 5*time.Millisecond)
	continuingElapsed.Store(int64(5 * time.Second))
	require.NoError(t, ackHolder.Commit(ctx))
	require.Error(t, <-done)
	require.Error(t, restrictionErr)
	savedIntent, err := st.LookupMemberAdmission(ctx, intent.Input.OperationID)
	require.NoError(t, err)
	require.Equal(t, "PREPARING", savedIntent.State)
	var memberCount, eventCount int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ProfileID).Scan(&memberCount))
	require.Zero(t, memberCount)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_member_admission_outbox WHERE operation_id=$1`, intent.Input.OperationID).Scan(&eventCount))
	require.Zero(t, eventCount)
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,outcome_proto FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, original.input.OperationID).Scan(&state, &outcome))
	require.Equal(t, "DECIDED", state)
	require.Nil(t, outcome)
	continuing, _ = signedApprovalOperationFixture(t, aliasRequest, account, profile)
	continuingProposal, err = st.PrepareJoinApprovalAlias(ctx, continuing)
	require.NoError(t, err)
	continuingDecision, err = seal(continuingProposal)
	require.NoError(t, err)
	_, recovery, err = st.ReadJoinApprovalAlias(ctx, continuingDecision)
	require.NoError(t, err)
	require.Equal(t, intent.Input.OperationID, *recovery.PublicationOperationID)
	restrictionErr = nil
	member, err := st.CompleteInteractiveMemberAdmissionWithPublicationCall(ctx, intent.Input.OperationID, roles, subjects, reviewers, nil, hook, continuing.call, continuing.request)
	t.Logf("fresh retry hook calls=%d, alias restriction=%v", hookCalls, restrictionErr)
	require.NoError(t, restrictionErr, "alias POST restriction hop")
	require.NoError(t, err)
	require.NotNil(t, member)
	require.Nil(t, restrictionErr)
	require.Equal(t, 2, hookCalls)
	originalAck, _, err := st.ReadJoinApprovalOperation(ctx, original)
	require.NoError(t, err)
	require.Equal(t, in.ProfileID.String(), originalAck.Result.SpaceMembership.ProfileId)
	// Historical alias response requires a NEW READ capability. No applicant
	// eligibility or native Role Ensure is requested by this branch.
	published, err := st.PrepareJoinApprovalAlias(ctx, continuing)
	require.NoError(t, err)
	none, readReq, err := published.OwnerRequests()
	require.NoError(t, err)
	require.Nil(t, none)
	require.Equal(t, authv1.SpaceAdmissionPhase_SPACE_ADMISSION_PHASE_READ, readReq.Phase)
	readSubject, err = subjects.Observe(ctx, readReq)
	require.NoError(t, err)
	readPermission, err = reviewers.Observe(ctx, readSubject, readReq)
	require.NoError(t, err)
	readDecision, err := published.Seal(nil, readPermission)
	require.NoError(t, err)
	aliasAck, noneRecovery, err := st.ReadJoinApprovalAlias(ctx, readDecision)
	require.NoError(t, err)
	require.Nil(t, noneRecovery)
	require.True(t, proto.Equal(originalAck, aliasAck))
	beforeEnsure := ensureCalls
	_, err = st.CompleteInteractiveMemberAdmissionWithPublicationCall(ctx, intent.Input.OperationID, nil, nil, nil, nil, hook, nil, nil)
	require.NoError(t, err)
	require.Equal(t, beforeEnsure, ensureCalls)
	require.Equal(t, 2, hookCalls, "completed native replay has zero hooks")
	allow = false
	currentReadSubject, err := subjects.Observe(ctx, readReq)
	require.NoError(t, err)
	_, err = reviewers.Observe(ctx, currentReadSubject, readReq)
	require.Error(t, err, "alias cannot borrow its previous successful result after permission removal")
	beforeOwner, beforePermission := ownerCalls, permissionCalls
	originalAck, _, err = st.ReadJoinApprovalOperation(ctx, original)
	require.NoError(t, err)
	require.NotNil(t, originalAck)
	require.Equal(t, beforeOwner, ownerCalls)
	require.Equal(t, beforePermission, permissionCalls, "only original exact retry keeps c7")
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,decision_basis_sha256,observed_basis_sha256,outcome_proto,publication_operation_id FROM space_join_operations WHERE actor_profile_id=$1 AND operation_id=$2`, profile, operation.input.OperationID).Scan(&state, &ownBasis, &observed, &outcome, &publication))
	require.Equal(t, "DECIDED", state)
	require.Nil(t, ownBasis)
	require.Equal(t, basisSHA[:], observed)
	require.Nil(t, outcome)
	require.Nil(t, publication)
}
