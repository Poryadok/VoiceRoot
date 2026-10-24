package grpcsvc

import (
	"context"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/store"
)

type deletionAuthClientStub struct {
	authv1.AuthServiceClient
	consume func(context.Context, *authv1.ConsumeSpaceDeletionProofRequest) (*authv1.ConsumeSpaceDeletionProofResponse, error)
	lookup  func(context.Context, *authv1.GetSpaceDeletionProofReceiptRequest) (*authv1.GetSpaceDeletionProofReceiptResponse, error)
}

func (c *deletionAuthClientStub) ConsumeSpaceDeletionProof(ctx context.Context, req *authv1.ConsumeSpaceDeletionProofRequest, _ ...grpc.CallOption) (*authv1.ConsumeSpaceDeletionProofResponse, error) {
	return c.consume(ctx, req)
}

func (c *deletionAuthClientStub) GetSpaceDeletionProofReceipt(ctx context.Context, req *authv1.GetSpaceDeletionProofReceiptRequest, _ ...grpc.CallOption) (*authv1.GetSpaceDeletionProofReceiptResponse, error) {
	return c.lookup(ctx, req)
}

func TestSpaceDeletionProofConsumeRecoversReceiptAndStoresDeterministicResponse(t *testing.T) {
	issuer, key := newOwnershipTestIssuer(t)
	accountID, profileID, spaceID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	proof, name, epoch := "opaque-deletion-proof", "Exact Space Name", int64(8)
	request := &spacev1.DeleteSpaceRequest{SpaceId: spaceID.String(), OperationId: operationID.String(), ConfirmationName: name, Proof: proof}
	response := validDeletionProofResponse(accountID, profileID, epoch, spaceID, operationID, name, proof)
	client := &deletionAuthClientStub{}
	client.consume = func(ctx context.Context, req *authv1.ConsumeSpaceDeletionProofRequest) (*authv1.ConsumeSpaceDeletionProofResponse, error) {
		require.Equal(t, uint32(1), req.GetProtocolVersion())
		require.Equal(t, accountID.String(), req.GetAccountId())
		require.Equal(t, profileID.String(), req.GetProfileId())
		require.Equal(t, epoch, req.GetSessionEpoch())
		require.Equal(t, spaceID.String(), req.GetSpaceId())
		require.Equal(t, operationID.String(), req.GetOperationId())
		require.Equal(t, name, req.GetConfirmationName())
		require.Equal(t, proof, req.GetProof())
		verifyOwnershipCredential(t, ctx, req, authv1.AuthService_ConsumeSpaceDeletionProof_FullMethodName, key)
		return nil, context.DeadlineExceeded
	}
	client.lookup = func(ctx context.Context, req *authv1.GetSpaceDeletionProofReceiptRequest) (*authv1.GetSpaceDeletionProofReceiptResponse, error) {
		nameDigest, proofDigest := sha256.Sum256([]byte(name)), sha256.Sum256([]byte(proof))
		require.Equal(t, uint32(1), req.GetProtocolVersion())
		require.Equal(t, accountID.String(), req.GetAccountId())
		require.Equal(t, profileID.String(), req.GetProfileId())
		require.Equal(t, epoch, req.GetSessionEpoch())
		require.Equal(t, spaceID.String(), req.GetSpaceId())
		require.Equal(t, operationID.String(), req.GetOperationId())
		require.Equal(t, nameDigest[:], req.GetConfirmationNameSha256())
		require.Equal(t, proofDigest[:], req.GetProofDigestSha256())
		verifyOwnershipCredential(t, ctx, req, authv1.AuthService_GetSpaceDeletionProofReceipt_FullMethodName, key)
		return &authv1.GetSpaceDeletionProofReceiptResponse{Receipt: response.GetReceipt()}, nil
	}
	service := &SpaceGRPC{PrincipalIssuer: issuer, OwnershipAuth: client}
	wrapped, digest, err := service.consumeSpaceDeletionProof(context.Background(), accountID, profileID, epoch, request)
	require.NoError(t, err)
	var stored authv1.ConsumeSpaceDeletionProofResponse
	require.NoError(t, proto.Unmarshal(wrapped, &stored))
	require.True(t, proto.Equal(response, &stored))
	expected, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	require.NoError(t, err)
	require.Equal(t, expected, wrapped, "saved evidence is the exact typed Auth response")
	require.Equal(t, lifecycleEvidenceHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", wrapped), digest)
}

func TestSpaceDeletionProofConsumedResponsePersistsInRealStore(t *testing.T) {
	if testing.Short() {
		t.Skip("requires test-owned PostgreSQL")
	}
	ctx := context.Background()
	account, owner, operation := uuid.New(), uuid.New(), uuid.New()
	pool, row := nativeLifecycleProofFixture(t, ctx, owner, "typed proof persistence")
	st := &store.SpaceStore{Pool: pool}
	const epoch = int64(7)
	const proof = "test-owned opaque proof"
	req := &spacev1.DeleteSpaceRequest{SpaceId: row.ID.String(), OperationId: operation.String(), ConfirmationName: row.Name, Proof: proof}
	response := validDeletionProofResponse(account, owner, epoch, row.ID, operation, row.Name, proof)
	missingRaw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	require.NoError(t, err)
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, owner, operation, missingRaw, lifecycleEvidenceHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", missingRaw))
	require.ErrorIs(t, err, pgx.ErrNoRows, "a receipt cannot create its missing pending operation")
	_, err = st.ReserveLifecycleSchedule(ctx, account, owner, epoch, req)
	require.NoError(t, err)
	issuer, _ := newOwnershipTestIssuer(t)
	service := &SpaceGRPC{PrincipalIssuer: issuer, OwnershipAuth: &deletionAuthClientStub{consume: func(context.Context, *authv1.ConsumeSpaceDeletionProofRequest) (*authv1.ConsumeSpaceDeletionProofResponse, error) {
		return response, nil
	}}}
	raw, digest, err := service.consumeSpaceDeletionProof(ctx, account, owner, epoch, req)
	require.NoError(t, err)
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, uuid.New(), operation, raw, digest)
	require.ErrorIs(t, err, store.ErrLifecycleConflict, "a foreign actor cannot persist this operation's receipt")
	for _, change := range []string{"foreign operation", "changed binding"} {
		invalid := proto.Clone(response).(*authv1.ConsumeSpaceDeletionProofResponse)
		if change == "foreign operation" {
			invalid.Receipt.OperationId = uuid.NewString()
		} else {
			invalid.Receipt.BindingSha256[0] ^= 0xff
		}
		invalidRaw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(invalid)
		require.NoError(t, err)
		_, err = st.RecordLifecycleDeletionProofReceipt(ctx, owner, operation, invalidRaw, lifecycleEvidenceHash("voice.auth.v1.ConsumeSpaceDeletionProofResponse", invalidRaw))
		require.ErrorIs(t, err, store.ErrLifecycleEvidenceInvalid, change)
	}
	saved, err := st.LifecycleDeletionProofRecorded(ctx, row.ID)
	require.NoError(t, err)
	require.False(t, saved, "rejected actor/operation/binding attempts leave the proof pending")
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, owner, operation, raw, digest)
	require.NoError(t, err, "real store must accept the validated typed Auth response")
	_, err = st.RecordLifecycleDeletionProofReceipt(ctx, owner, operation, raw, digest)
	require.NoError(t, err, "exact response retry must be durable and idempotent")
}

// Seed only the genuine legacy Space before native enforcement. The actual
// trusted45->50 transition must precede enrollment and serving grants; a
// qualified audit45 component alone cannot establish this current fixture.
func nativeLifecycleProofFixture(t *testing.T, ctx context.Context, owner uuid.UUID, name string) (*pgxpool.Pool, *store.SpaceRow) {
	return nativeLifecycleProofFixtureWithSeed(t, ctx, owner, name, nil, false)
}

func nativeLifecycleProofFixtureWithSeed(t *testing.T, ctx context.Context, owner uuid.UUID, name string, seed func(*store.SpaceStore, *store.SpaceRow), currentCatalog bool) (*pgxpool.Pool, *store.SpaceRow) {
	t.Helper()
	directory := filepath.Join(repoRoot(t), "src/backend/migrations/space_db")
	db := auditMigrationFixture(t, ctx, directory)
	db.Run(true, "goto", "34")
	installer := &store.SpaceStore{Pool: db.Pool}
	row, err := installer.CreateSpace(ctx, owner, name, "", "private")
	require.NoError(t, err)
	if seed != nil {
		seed(installer, row)
	}
	db.Run(true, "goto", "44")
	f44, err := installer.InspectNativeInstallerSource(ctx, 44)
	require.NoError(t, err, "full44 source and the real M44-created seal are required")
	db.Run(true, "goto", "45")
	r45, err := installer.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	require.True(t, r45.Changed)
	require.Equal(t, f44.SealSHA256, r45.OldSealSHA256)
	require.Equal(t, f44.DatabaseOID, r45.DatabaseOID)
	require.Equal(t, f44.DatabaseSystemID, r45.DatabaseSystemID)
	require.Equal(t, f44.DatabaseName, r45.DatabaseName)
	require.Equal(t, f44.InstallerRole, r45.InstallerRole)
	require.Equal(t, f44.Epoch, r45.Epoch)
	f45, err := installer.InspectNativeInstallerSource(ctx, 45)
	require.NoError(t, err)
	require.Equal(t, r45.NewSealSHA256, f45.SealSHA256)
	db.Run(true, "goto", "50")
	r50, err := installer.TransitionNativeMembershipCatalog50(ctx)
	require.NoError(t, err)
	require.True(t, r50.Changed)
	require.Equal(t, f45.SealSHA256, r50.OldSealSHA256)
	require.Equal(t, f45.DatabaseOID, r50.DatabaseOID)
	require.Equal(t, f45.DatabaseSystemID, r50.DatabaseSystemID)
	require.Equal(t, f45.DatabaseName, r50.DatabaseName)
	require.Equal(t, f45.InstallerRole, r50.InstallerRole)
	require.Equal(t, f45.Epoch, r50.Epoch)
	f50, err := installer.InspectNativeInstallerSource(ctx, 50)
	require.NoError(t, err)
	require.Equal(t, r50.NewSealSHA256, f50.SealSHA256)
	require.NoError(t, installer.InitializeNativeMembership(ctx, row.ID))
	var complete, drained bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state='COMPLETE' FROM native_membership_guard.spaces WHERE space_id=$1`, row.ID).Scan(&complete))
	require.True(t, complete)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`).Scan(&drained))
	require.True(t, drained)
	final, err := installer.InspectNativeInstallerSource(ctx, 50)
	require.NoError(t, err)
	require.Equal(t, f50, final, "enrollment cannot replace the catalog seal or target")
	if currentCatalog {
		db.Pool = migrateModerationFixture51(t, ctx, db.Pool, directory)
	}
	role := "proof_serve_" + uuid.New().String()[:8]
	_, err = db.Pool.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD 'isolated-proof-serving'")
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+pgx.Identifier{role}.Sanitize()+"; GRANT SELECT,INSERT,UPDATE,DELETE ON ALL TABLES IN SCHEMA public TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	config := db.Pool.Config().Copy()
	config.ConnConfig.User, config.ConnConfig.Password = role, "isolated-proof-serving"
	serving, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		serving.Close()
		_, _ = db.Pool.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{role}.Sanitize())
		_, _ = db.Pool.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	})
	unframedName := "unframed proof fixture " + uuid.NewString()
	_, err = (&store.SpaceStore{Pool: serving}).CreateSpace(ctx, owner, unframedName, "", "private")
	var guarded *pgconn.PgError
	require.ErrorAs(t, err, &guarded)
	require.Equal(t, "55000", guarded.Code, "fixture repair must preserve native birth enforcement")
	var created int
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM spaces WHERE name=$1`, unframedName).Scan(&created))
	require.Zero(t, created)
	t.Logf("qualified native50 base database=%s oid=%d epoch=%s old45=%s new50=%s upgraded51=%t COMPLETE=true frames=0 serving=%s rawCreate=55000/no-effect", final.DatabaseName, final.DatabaseOID, final.Epoch, r50.OldSealSHA256, r50.NewSealSHA256, currentCatalog, role)
	return serving, row
}

func TestSpaceDeletionProofConsumeRejectsReceiptWithChangedBinding(t *testing.T) {
	issuer, _ := newOwnershipTestIssuer(t)
	accountID, profileID, spaceID, operationID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	proof, name, epoch := "opaque-deletion-proof", "Exact Space Name", int64(8)
	response := validDeletionProofResponse(accountID, profileID, epoch, spaceID, operationID, name, proof)
	response.Receipt.BindingSha256 = append([]byte(nil), response.Receipt.BindingSha256...)
	response.Receipt.BindingSha256[0] ^= 0xff
	service := &SpaceGRPC{
		PrincipalIssuer: issuer,
		OwnershipAuth: &deletionAuthClientStub{consume: func(context.Context, *authv1.ConsumeSpaceDeletionProofRequest) (*authv1.ConsumeSpaceDeletionProofResponse, error) {
			return response, nil
		}},
	}
	_, _, err := service.consumeSpaceDeletionProof(context.Background(), accountID, profileID, epoch, &spacev1.DeleteSpaceRequest{
		SpaceId: spaceID.String(), OperationId: operationID.String(), ConfirmationName: name, Proof: proof,
	})
	require.Error(t, err)
}

func validDeletionProofResponse(accountID, profileID uuid.UUID, epoch int64, spaceID, operationID uuid.UUID, name, proof string) *authv1.ConsumeSpaceDeletionProofResponse {
	nameDigest, proofDigest := sha256.Sum256([]byte(name)), sha256.Sum256([]byte(proof))
	factors := []authv1.VerifiedFactor{authv1.VerifiedFactor_VERIFIED_FACTOR_PASSWORD, authv1.VerifiedFactor_VERIFIED_FACTOR_TOTP}
	binding := &authv1.SpaceDeletionProofBinding{
		ProtocolVersion: 1, AccountId: accountID.String(), ProfileId: profileID.String(), SessionEpoch: epoch,
		SpaceId: spaceID.String(), OperationId: operationID.String(), ConfirmationNameSha256: nameDigest[:],
		ProofDigestSha256: proofDigest[:], VerifiedFactors: factors, Purpose: authv1.ProofPurpose_PROOF_PURPOSE_SPACE_DELETE,
	}
	bindingBytes, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(binding)
	bindingDigest := lifecycleEvidenceHash("voice.auth.v1.SpaceDeletionProofBinding", bindingBytes)
	return &authv1.ConsumeSpaceDeletionProofResponse{Receipt: &authv1.SpaceDeletionProofReceipt{
		ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: operationID.String(), BindingSha256: bindingDigest[:],
		ConfirmationNameSha256: nameDigest[:], ConsumedAt: timestamppb.New(time.Now().UTC().Truncate(time.Microsecond)), VerifiedFactors: factors,
	}}
}
