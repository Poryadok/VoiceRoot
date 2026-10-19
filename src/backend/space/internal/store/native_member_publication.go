package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
)

const nativeMemberPublicationDomain = "voice.space.native-membership-publication.v1\x00"

// The GIS adapter authenticates the producer before entering this transaction.
// A native proof records membership provenance, never a default-role receipt.
func publishCommunityBootstrapOwner(ctx context.Context, tx pgx.Tx, space uuid.UUID, in CommunityBootstrapSpaceInput, requestHash []byte) error {
	if len(requestHash) != 32 {
		return ErrMemberAdmissionConflict
	}
	var published time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&published); err != nil {
		return err
	}
	account := in.OwnerAccountID.String()
	binding := &spacev1.NativeMembershipPublicationBinding{ProtocolVersion: 1, PublicationOperationId: in.OperationID.String(), SpaceId: space.String(), ProfileId: in.OwnerProfileID.String(), OrdinaryGeneration: 1,
		Kind: spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE, InitiatingService: "gameintegration", InitiatingAccountId: &account, InitiatingProfileId: in.OwnerProfileID.String(), RequestSha256: append([]byte(nil), requestHash...), ProducerOperationId: in.OperationID.String(), ProducerEvidenceSha256: append([]byte(nil), requestHash...), ApplicationId: proto.String(in.ApplicationID.String()), EnvironmentId: proto.String(in.EnvironmentID.String()), CorporationKey: proto.String(in.CorporationKey), PublishedAt: timestamppb.New(published)}
	return insertNativeOrdinaryPublication(ctx, tx, binding, in.OwnerAccountID)
}

func insertNativeOrdinaryPublication(ctx context.Context, tx pgx.Tx, binding *spacev1.NativeMembershipPublicationBinding, account uuid.UUID) error {
	if binding == nil || binding.GetOrdinaryGeneration() == 0 || binding.GetOrdinaryGeneration() > math.MaxInt64 || (binding.GetKind() != spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE && binding.GetKind() != spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE && binding.GetKind() != spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_RECOVERY) || binding.PublishedAt == nil || binding.PublishedAt.CheckValid() != nil || account == uuid.Nil {
		return ErrMemberAdmissionConflict
	}
	operation, err := canonicalLifecycleUUID(binding.GetPublicationOperationId())
	if err != nil {
		return err
	}
	space, err := canonicalLifecycleUUID(binding.GetSpaceId())
	if err != nil {
		return err
	}
	profile, err := canonicalLifecycleUUID(binding.GetProfileId())
	if err != nil {
		return err
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(binding)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(append([]byte(nativeMemberPublicationDomain), raw...))
	if _, err = tx.Exec(ctx, `INSERT INTO space_native_member_publications(operation_id,space_id,profile_id,generation,kind,binding_bytes,binding_sha256,state,published_at)
	 VALUES($1,$2,$3,$8,$4,$5,$6,'PUBLISHED',$7)`, operation, space, profile, int32(binding.Kind), raw, digest[:], binding.PublishedAt.AsTime(), binding.GetOrdinaryGeneration()); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO space_ordinary_member_heads(space_id,profile_id,generation,publication_id,state,account_id,publication_kind,native_publication_sha256)
	 VALUES($1,$2,$7,$3,'ACTIVE',$4,$5,$6) ON CONFLICT(space_id,profile_id) DO UPDATE SET
	 generation=EXCLUDED.generation,publication_id=EXCLUDED.publication_id,state='ACTIVE',legacy=false,
	 account_id=EXCLUDED.account_id,publication_kind=EXCLUDED.publication_kind,native_publication_sha256=EXCLUDED.native_publication_sha256,
	 admission_binding_sha256=NULL,role_receipt_sha256=NULL`, space, profile, operation, account, int32(binding.Kind), digest[:], binding.GetOrdinaryGeneration()); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_members(space_id,profile_id,membership_generation,admission_operation_id) VALUES($1,$2,$4,$3)`, space, profile, operation, binding.GetOrdinaryGeneration())
	return err
}

// The caller holds the shared Space fence and exact pending recovery row.
// Existing membership retains its original source; only an absent member gets
// a new native incarnation, and an obstructing inert candidate is retired first.
func publishCommunityRecoveryOwner(ctx context.Context, tx pgx.Tx, in CommunityOwnerRecoveryInput, receipt []byte) (bool, error) {
	if err := validateCommunityRecoveryReceipt(in, receipt); err != nil {
		return false, err
	}
	var member bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, in.SpaceID, in.ReplacementProfileID).Scan(&member); err != nil {
		return false, err
	}
	if member {
		authority, err := readOrdinaryMemberAuthority(ctx, tx, in.SpaceID, in.ReplacementProfileID)
		if err != nil {
			return false, err
		}
		if authority.GetState() != spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE {
			return false, ErrMemberAuthorityUnavailable
		}
		return false, nil
	}
	var generation, floor int64
	var state string
	var operation uuid.UUID
	err := tx.QueryRow(ctx, `SELECT generation,retired_through,state,publication_id FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2 FOR UPDATE`, in.SpaceID, in.ReplacementProfileID).Scan(&generation, &floor, &state, &operation)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if state == "ACTIVE" {
		return false, ErrMemberAuthorityUnavailable
	}
	if generation == math.MaxInt64 || floor == math.MaxInt64 {
		return false, ErrMemberAdmissionStale
	}
	if state == "PREPARING" {
		intent, _, err := scanMemberAdmission(tx.QueryRow(ctx, `SELECT `+memberAdmissionColumns+` FROM space_member_admission_intents WHERE operation_id=$1`, operation))
		if err != nil {
			return false, err
		}
		if intent.Input.SpaceID != in.SpaceID || intent.Input.ProfileID != in.ReplacementProfileID || intent.Generation != uint64(generation) || intent.State != "PREPARING" {
			return false, ErrMemberAuthorityUnavailable
		}
		if err = (&SpaceStore{tx: tx}).invalidateMemberAdmission(ctx, intent); err != nil {
			return false, err
		}
		generation++
		floor = generation
	}
	if generation == math.MaxInt64 || floor == math.MaxInt64 {
		return false, ErrMemberAdmissionStale
	}
	var published time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&published); err != nil {
		return false, err
	}
	request, err := communityOwnerRecoveryHash(in)
	if err != nil {
		return false, err
	}
	evidence := sha256.Sum256(receipt)
	account := in.ReplacementAccountID.String()
	binding := &spacev1.NativeMembershipPublicationBinding{ProtocolVersion: 1, PublicationOperationId: in.OperationID.String(), SpaceId: in.SpaceID.String(), ProfileId: in.ReplacementProfileID.String(), OrdinaryGeneration: uint64(max(generation, floor) + 1), Kind: spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_RECOVERY, InitiatingService: "gameintegration", InitiatingAccountId: &account, InitiatingProfileId: in.ReplacementProfileID.String(), RequestSha256: request[:], ProducerOperationId: in.OperationID.String(), ProducerEvidenceSha256: evidence[:], ApplicationId: proto.String(in.ApplicationID.String()), EnvironmentId: proto.String(in.EnvironmentID.String()), CorporationKey: proto.String(in.CorporationKey), PreviousOwnerProfileId: proto.String(in.PreviousOwnerProfileID.String()), PreviousOwnerAccountId: proto.String(in.PreviousOwnerAccountID.String()), ExpectedOwnerGeneration: proto.Uint64(uint64(in.ExpectedGeneration)), ResultOwnerGeneration: proto.Uint64(uint64(in.ExpectedGeneration + 1)), PublishedAt: timestamppb.New(published)}
	if err = insertNativeOrdinaryPublication(ctx, tx, binding, in.ReplacementAccountID); err != nil {
		return false, err
	}
	return true, nil
}

func validateCommunityRecoveryNativeProof(ctx context.Context, tx pgx.Tx, b *spacev1.NativeMembershipPublicationBinding) error {
	operation, err := canonicalLifecycleUUID(b.GetProducerOperationId())
	if err != nil {
		return ErrMemberAuthorityUnavailable
	}
	op, err := loadCommunityOwnerRecovery(ctx, tx, operation, false)
	if err != nil || validateCommunityOwnerRecovery(op.Input) != nil || op.Status != "succeeded" || op.ResultGeneration != op.Input.ExpectedGeneration+1 || validateCommunityRecoveryReceipt(op.Input, op.RoleReceipt) != nil {
		return ErrMemberAuthorityUnavailable
	}
	in := op.Input
	request, err := communityOwnerRecoveryHash(in)
	if err != nil {
		return ErrMemberAuthorityUnavailable
	}
	evidence := sha256.Sum256(op.RoleReceipt)
	if b.GetInitiatingService() != "gameintegration" || b.GetProducerOperationId() != b.GetPublicationOperationId() || b.GetPublicationOperationId() != in.OperationID.String() || b.GetSpaceId() != in.SpaceID.String() || b.GetProfileId() != in.ReplacementProfileID.String() || b.InitiatingAccountId == nil || b.GetInitiatingAccountId() != in.ReplacementAccountID.String() || b.GetInitiatingProfileId() != in.ReplacementProfileID.String() || !bytes.Equal(b.GetRequestSha256(), request[:]) || !bytes.Equal(b.GetProducerEvidenceSha256(), evidence[:]) || b.GetApplicationId() != in.ApplicationID.String() || b.GetEnvironmentId() != in.EnvironmentID.String() || b.GetCorporationKey() != in.CorporationKey || b.GetPreviousOwnerProfileId() != in.PreviousOwnerProfileID.String() || b.GetPreviousOwnerAccountId() != in.PreviousOwnerAccountID.String() || b.ExpectedOwnerGeneration == nil || b.GetExpectedOwnerGeneration() != uint64(in.ExpectedGeneration) || b.ResultOwnerGeneration == nil || b.GetResultOwnerGeneration() != uint64(op.ResultGeneration) || b.GetBotId() != "" || b.GetInstallationId() != "" || b.PublishedAt == nil || b.PublishedAt.CheckValid() != nil || len(b.PublishedAt.ProtoReflect().GetUnknown()) != 0 {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}

// Historical bootstrap replay may recover Role's original owner baseline only
// while that exact native member and owner are still current. It never repairs
// membership after leave, transfer or purge.
func (s *SpaceStore) IsCommunityBootstrapPublicationCurrent(ctx context.Context, operation, space uuid.UUID) (bool, error) {
	if s == nil || s.Pool == nil || operation == uuid.Nil || space == uuid.Nil {
		return false, ErrMemberAdmissionConflict
	}
	if s.tx == nil {
		return withOwnershipScopeValue(s, ctx, []uuid.UUID{space}, func(scoped *SpaceStore) (bool, error) {
			return scoped.IsCommunityBootstrapPublicationCurrent(ctx, operation, space)
		})
	}
	var current bool
	err := s.db().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_native_member_publications p
	 JOIN space_ordinary_member_heads h ON h.space_id=p.space_id AND h.profile_id=p.profile_id AND h.generation=p.generation AND h.publication_id=p.operation_id AND h.native_publication_sha256=p.binding_sha256 AND h.state='ACTIVE'
	 JOIN space_members m ON m.space_id=h.space_id AND m.profile_id=h.profile_id AND m.membership_generation=h.generation AND m.admission_operation_id=h.publication_id
	 JOIN spaces s ON s.id=h.space_id AND s.owner_profile_id=h.profile_id
	 WHERE p.operation_id=$1 AND p.space_id=$2 AND p.kind=4 AND p.state='PUBLISHED')`, operation, space).Scan(&current)
	return current, err
}

func validateCommunityBootstrapNativeProof(ctx context.Context, tx pgx.Tx, binding *spacev1.NativeMembershipPublicationBinding) error {
	if binding.GetInitiatingService() != "gameintegration" || binding.GetInitiatingProfileId() != binding.GetProfileId() || binding.GetProducerOperationId() != binding.GetPublicationOperationId() || len(binding.GetRequestSha256()) != 32 || !bytes.Equal(binding.GetRequestSha256(), binding.GetProducerEvidenceSha256()) || binding.InitiatingAccountId == nil || binding.PublishedAt == nil || binding.PublishedAt.CheckValid() != nil || len(binding.PublishedAt.ProtoReflect().GetUnknown()) != 0 {
		return ErrMemberAuthorityUnavailable
	}
	for _, id := range []string{binding.GetInitiatingAccountId(), binding.GetApplicationId(), binding.GetEnvironmentId()} {
		if _, err := canonicalLifecycleUUID(id); err != nil {
			return ErrMemberAuthorityUnavailable
		}
	}
	if binding.GetCorporationKey() == "" || binding.GetPreviousOwnerProfileId() != "" || binding.GetPreviousOwnerAccountId() != "" || binding.GetExpectedOwnerGeneration() != 0 || binding.GetResultOwnerGeneration() != 0 || binding.GetBotId() != "" || binding.GetInstallationId() != "" {
		return ErrMemberAuthorityUnavailable
	}
	var verified bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM community_bootstrap_operations WHERE operation_id=$1 AND space_id=$2 AND owner_profile_id=$3 AND owner_account_id=$4
	 AND application_id=$5 AND environment_id=$6 AND corporation_key=$7 AND request_hash=$8 AND status='succeeded')`, binding.GetProducerOperationId(), binding.GetSpaceId(), binding.GetProfileId(), binding.GetInitiatingAccountId(), binding.GetApplicationId(), binding.GetEnvironmentId(), binding.GetCorporationKey(), binding.GetProducerEvidenceSha256()).Scan(&verified)
	if err != nil || !verified {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}
