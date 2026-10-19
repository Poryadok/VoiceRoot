package store

import (
	"bytes"
	"context"
	"crypto/sha256"
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
	if binding == nil || binding.GetOrdinaryGeneration() != 1 || (binding.GetKind() != spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE && binding.GetKind() != spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE) || binding.PublishedAt == nil || binding.PublishedAt.CheckValid() != nil || account == uuid.Nil {
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
	 VALUES($1,$2,$3,1,$4,$5,$6,'PUBLISHED',$7)`, operation, space, profile, int32(binding.Kind), raw, digest[:], binding.PublishedAt.AsTime()); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO space_ordinary_member_heads(space_id,profile_id,generation,publication_id,state,account_id,publication_kind,native_publication_sha256)
	 VALUES($1,$2,1,$3,'ACTIVE',$4,$5,$6)`, space, profile, operation, account, int32(binding.Kind), digest[:]); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_members(space_id,profile_id,membership_generation,admission_operation_id) VALUES($1,$2,1,$3)`, space, profile, operation)
	return err
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
