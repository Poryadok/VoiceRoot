package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/lifecycleprincipal"
)

type nativeCreatorWitness struct {
	operation, space, account, profile, request uuid.UUID
	jti                                         string
	epoch                                       int64
	requestHash                                 []byte
	issued, expires                             time.Time
}

// Fixed framing: operation/Space/account/profile/request UUID16 values, JTI20,
// epochBE64, requestSHA256, issuedUnixBE64, expiresUnixBE64. The domain fixes
// Gateway/space/CreateSpace/regular, never arbitrary caller-supplied authority.
func creatorWitnessDigest(w nativeCreatorWitness) ([32]byte, error) {
	for _, id := range []uuid.UUID{w.operation, w.space, w.account, w.profile, w.request} {
		if id == uuid.Nil {
			return [32]byte{}, ErrMemberAuthorityUnavailable
		}
	}
	if !validCreatorJWTID(w.jti) || w.epoch <= 0 || len(w.requestHash) != 32 || w.issued.IsZero() || !w.expires.After(w.issued) || w.expires.Sub(w.issued) > 30*time.Second || w.issued.Nanosecond() != 0 || w.expires.Nanosecond() != 0 {
		return [32]byte{}, ErrMemberAuthorityUnavailable
	}
	data := []byte("voice.space.creator-native-proof.v1\x00")
	for _, id := range []uuid.UUID{w.operation, w.space, w.account, w.profile, w.request} {
		data = append(data, id[:]...)
	}
	jti, _ := base64.RawURLEncoding.DecodeString(w.jti)
	data = append(data, jti...)
	data = binary.BigEndian.AppendUint64(data, uint64(w.epoch))
	data = append(data, w.requestHash...)
	data = binary.BigEndian.AppendUint64(data, uint64(w.issued.Unix()))
	data = binary.BigEndian.AppendUint64(data, uint64(w.expires.Unix()))
	return sha256.Sum256(data), nil
}

// CreateSpaceWithCreator consumes the private verified listener capability.
// The final Auth read occurred outside this transaction; body/expiry are checked
// again locally after acquiring the fence and before commit. No new legacy0 or
// fabricated default-role readiness can enter the native owner path.
func (s *SpaceStore) CreateSpaceWithCreator(ctx context.Context, creator *lifecycleprincipal.VerifiedCreator, request *spacev1.CreateSpaceRequest) (*SpaceRow, error) {
	if s == nil || s.Pool == nil || s.tx != nil || request == nil {
		return nil, ErrMemberAdmissionConflict
	}
	request = proto.Clone(request).(*spacev1.CreateSpaceRequest)
	actor, err := creator.ActorFor(request)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return nil, ErrMemberAdmissionConflict
	}
	visibility := strings.TrimSpace(request.Visibility)
	if visibility == "" {
		visibility = "private"
	}
	requestHash, err := hex.DecodeString(strings.TrimPrefix(actor.RequestHash, "sha256:"))
	if err != nil || len(requestHash) != 32 {
		return nil, ErrMemberAdmissionConflict
	}
	w := nativeCreatorWitness{operation: uuid.New(), space: uuid.New(), account: uuid.MustParse(actor.AccountID), profile: uuid.MustParse(actor.ProfileID), request: uuid.MustParse(actor.RequestID), epoch: actor.SessionEpoch, requestHash: requestHash, issued: actor.IssuedAt.UTC(), expires: actor.ExpiresAt.UTC()}
	w.jti = actor.JWTID
	if !validCreatorJWTID(w.jti) {
		return nil, ErrMemberAdmissionConflict
	}
	evidence, err := creatorWitnessDigest(w)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{w.space}); err != nil {
		return nil, err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, err
	}
	if _, err = creator.ActorFor(request); err != nil {
		return nil, err
	}
	scoped := *s
	scoped.tx = tx
	schema, err := scoped.memberDecisionSchema(ctx)
	if err != nil {
		return nil, err
	}
	var effective *effectiveMemberMutations
	if schema == 44 || schema == 45 || schema == 46 {
		effective, err = scoped.captureBornCreatorEffectiveMutation(ctx, w.space, w.profile)
		if err != nil {
			return nil, err
		}
	}
	row, err := scanSpaceRow(tx.QueryRow(ctx, `INSERT INTO spaces(id,name,description,visibility,owner_profile_id,member_count)
	 VALUES($1,$2,$3,$4,$5,1) RETURNING `+spaceSelectColumns, w.space, name, request.Description, visibility, w.profile))
	if err != nil {
		return nil, err
	}
	if schema != 44 && schema != 45 && schema != 46 {
		effective, err = scoped.captureNativeEffectiveMemberMutation(ctx, w.space, []uuid.UUID{w.profile})
		if err != nil {
			return nil, err
		}
	}
	var published time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&published); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_native_creator_operations(operation_id,space_id,account_id,profile_id,session_epoch,request_id,jti,request_sha256,credential_issued_at,credential_expires_at,published_at)
	 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, w.operation, w.space, w.account, w.profile, w.epoch, w.request, w.jti, w.requestHash, w.issued, w.expires, published)
	if err != nil {
		return nil, err
	}
	account := w.account.String()
	binding := &spacev1.NativeMembershipPublicationBinding{ProtocolVersion: 1, PublicationOperationId: w.operation.String(), SpaceId: w.space.String(), ProfileId: w.profile.String(), OrdinaryGeneration: 1, Kind: spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_OWNER_CREATE, InitiatingService: "gateway", InitiatingAccountId: &account, InitiatingProfileId: w.profile.String(), RequestSha256: w.requestHash, ProducerOperationId: w.operation.String(), ProducerEvidenceSha256: evidence[:], PublishedAt: timestamppb.New(published)}
	if err = insertNativeOrdinaryPublication(ctx, tx, binding, w.account); err != nil {
		return nil, err
	}
	if effective != nil {
		if err = scoped.finishEffectiveMemberMutations(ctx, effective); err != nil {
			return nil, err
		}
	}
	if _, err = creator.ActorFor(request); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return row, nil
}

func validateCreatorNativeProof(ctx context.Context, tx pgx.Tx, b *spacev1.NativeMembershipPublicationBinding) error {
	if b.GetInitiatingService() != "gateway" || b.GetInitiatingProfileId() != b.GetProfileId() || b.GetProducerOperationId() != b.GetPublicationOperationId() || b.InitiatingAccountId == nil || len(b.GetRequestSha256()) != 32 || len(b.GetProducerEvidenceSha256()) != 32 || b.PublishedAt == nil || b.PublishedAt.CheckValid() != nil || len(b.PublishedAt.ProtoReflect().GetUnknown()) != 0 || b.GetApplicationId() != "" || b.GetEnvironmentId() != "" || b.GetCorporationKey() != "" || b.GetPreviousOwnerProfileId() != "" || b.GetPreviousOwnerAccountId() != "" || b.GetExpectedOwnerGeneration() != 0 || b.GetResultOwnerGeneration() != 0 || b.GetBotId() != "" || b.GetInstallationId() != "" {
		return ErrMemberAuthorityUnavailable
	}
	var w nativeCreatorWitness
	var published time.Time
	err := tx.QueryRow(ctx, `SELECT operation_id,space_id,account_id,profile_id,session_epoch,request_id,jti,request_sha256,credential_issued_at,credential_expires_at,published_at
	 FROM space_native_creator_operations WHERE operation_id=$1 AND space_id=$2 AND profile_id=$3`, b.GetProducerOperationId(), b.GetSpaceId(), b.GetProfileId()).Scan(&w.operation, &w.space, &w.account, &w.profile, &w.epoch, &w.request, &w.jti, &w.requestHash, &w.issued, &w.expires, &published)
	if err != nil || b.GetInitiatingAccountId() != w.account.String() || !bytes.Equal(b.GetRequestSha256(), w.requestHash) || !published.Equal(b.PublishedAt.AsTime()) || !published.Before(w.expires) || published.Before(w.issued.Add(-5*time.Second)) {
		return ErrMemberAuthorityUnavailable
	}
	digest, err := creatorWitnessDigest(w)
	if err != nil || !bytes.Equal(digest[:], b.GetProducerEvidenceSha256()) {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}
