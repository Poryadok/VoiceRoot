package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
)

func botInstallationEvidence(in botmembershipprincipal.InstallationBinding, membershipGeneration uint64, requestSHA []byte) ([32]byte, error) {
	if in.Remove || in.OperationID != in.InstallOperationID || in.Generation == 0 || in.Generation > math.MaxInt64 || membershipGeneration == 0 || membershipGeneration > math.MaxInt64 || len(requestSHA) != 32 {
		return [32]byte{}, ErrMemberAdmissionConflict
	}
	raw := []byte("voice.space.bot-installation-native-proof.v1\x00")
	for _, id := range []uuid.UUID{in.OperationID, in.BotID, in.InstallationID, in.SpaceID, in.ProfileID} {
		if id == uuid.Nil {
			return [32]byte{}, ErrMemberAdmissionConflict
		}
		raw = append(raw, id[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, in.Generation)
	raw = binary.BigEndian.AppendUint64(raw, membershipGeneration)
	raw = append(raw, requestSHA...)
	return sha256.Sum256(raw), nil
}

// The caller owns the Space fence, qualified catalog and effective-history
// capture. This primitive performs no remote I/O and commits nothing itself.
func publishNativeBotInstallation(ctx context.Context, tx pgx.Tx, in botmembershipprincipal.InstallationBinding, requestSHA []byte) error {
	if _, err := botInstallationEvidence(in, 1, requestSHA); err != nil {
		return err
	}
	var savedSpace, savedBot, savedProfile, savedInstall uuid.UUID
	var savedGeneration uint64
	var savedRequest []byte
	var savedState string
	err := tx.QueryRow(ctx, `SELECT space_id,bot_id,profile_id,installation_id,generation,add_request_sha256,state FROM space_bot_installation_attempts WHERE install_operation_id=$1`, in.OperationID).Scan(&savedSpace, &savedBot, &savedProfile, &savedInstall, &savedGeneration, &savedRequest, &savedState)
	if err == nil {
		if savedSpace != in.SpaceID || savedBot != in.BotID || savedProfile != in.ProfileID || savedInstall != in.InstallationID || savedGeneration != in.Generation {
			return ErrMemberAdmissionConflict
		}
		if savedState == "CANCELED" {
			return ErrMemberAdmissionStale
		}
		if !bytes.Equal(savedRequest, requestSHA) {
			return ErrMemberAdmissionConflict
		}
		authority, err := readOrdinaryMemberAuthority(ctx, tx, in.SpaceID, in.ProfileID)
		if err != nil {
			return err
		}
		if authority.GetState() != spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE || authority.GetPublishedBinding().GetOrdinary().GetPublicationIdentity() != in.OperationID.String() {
			return ErrMemberAdmissionStale
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var predecessor, floor uint64
	var predecessorState string
	var oldInstall, oldOp uuid.UUID
	err = tx.QueryRow(ctx, `SELECT generation,retired_through,state,installation_id,install_operation_id FROM space_bot_installation_heads WHERE space_id=$1 AND bot_id=$2 FOR UPDATE`, in.SpaceID, in.BotID).Scan(&predecessor, &floor, &predecessorState, &oldInstall, &oldOp)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if (errors.Is(err, pgx.ErrNoRows) && in.Generation != 1) || (err == nil && (predecessorState != "CANCELED" || predecessor == math.MaxInt64 || in.Generation != predecessor+1 || floor != predecessor || oldInstall == in.InstallationID || oldOp == in.OperationID)) {
		return ErrMemberAdmissionStale
	}
	var generation, retired uint64
	var state string
	err = tx.QueryRow(ctx, `SELECT generation,retired_through,state FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2 FOR UPDATE`, in.SpaceID, in.ProfileID).Scan(&generation, &retired, &state)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if state == "ACTIVE" || state == "PREPARING" {
		return ErrMemberAuthorityUnavailable
	}
	var member bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_members WHERE space_id=$1 AND profile_id=$2)`, in.SpaceID, in.ProfileID).Scan(&member); err != nil {
		return err
	}
	if member {
		return ErrMemberAuthorityUnavailable
	}
	if max(generation, retired) == math.MaxInt64 {
		return ErrMemberAdmissionStale
	}
	generation = max(generation, retired) + 1
	evidence, err := botInstallationEvidence(in, generation, requestSHA)
	if err != nil {
		return err
	}
	var published time.Time
	if err = tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&published); err != nil {
		return err
	}
	binding := &spacev1.NativeMembershipPublicationBinding{ProtocolVersion: 1, PublicationOperationId: in.OperationID.String(), SpaceId: in.SpaceID.String(), ProfileId: in.ProfileID.String(), OrdinaryGeneration: generation, Kind: spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_BOT_INSTALL, InitiatingService: "bot", InitiatingProfileId: in.ProfileID.String(), RequestSha256: append([]byte(nil), requestSHA...), ProducerOperationId: in.OperationID.String(), ProducerEvidenceSha256: evidence[:], BotId: proto.String(in.BotID.String()), InstallationId: proto.String(in.InstallationID.String()), PublishedAt: timestamppb.New(published)}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(binding)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(append([]byte(nativeMemberPublicationDomain), raw...))
	if _, err = tx.Exec(ctx, `INSERT INTO space_bot_installation_attempts(install_operation_id,installation_id,space_id,bot_id,profile_id,generation,state,membership_generation,add_request_sha256,native_publication_sha256,published_at) VALUES($1,$2,$3,$4,$5,$6,'PUBLISHED',$7,$8,$9,$10)`, in.OperationID, in.InstallationID, in.SpaceID, in.BotID, in.ProfileID, in.Generation, generation, requestSHA, digest[:], published); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO space_native_member_publications(operation_id,space_id,profile_id,generation,kind,binding_bytes,binding_sha256,state,published_at) VALUES($1,$2,$3,$4,6,$5,$6,'PUBLISHED',$7)`, in.OperationID, in.SpaceID, in.ProfileID, generation, raw, digest[:], published); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO space_ordinary_member_heads(space_id,profile_id,generation,publication_id,state,publication_kind,native_publication_sha256) VALUES($1,$2,$3,$4,'ACTIVE',6,$5) ON CONFLICT(space_id,profile_id) DO UPDATE SET generation=EXCLUDED.generation,publication_id=EXCLUDED.publication_id,state='ACTIVE',legacy=false,account_id=NULL,publication_kind=6,native_publication_sha256=EXCLUDED.native_publication_sha256,admission_binding_sha256=NULL,role_receipt_sha256=NULL`, in.SpaceID, in.ProfileID, generation, in.OperationID, digest[:]); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO space_members(space_id,profile_id,membership_generation,admission_operation_id) VALUES($1,$2,$3,$4)`, in.SpaceID, in.ProfileID, generation, in.OperationID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_bot_installation_heads(space_id,bot_id,profile_id,installation_id,install_operation_id,generation,retired_through,membership_generation,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'INSTALLED') ON CONFLICT(space_id,bot_id) DO UPDATE SET profile_id=EXCLUDED.profile_id,installation_id=EXCLUDED.installation_id,install_operation_id=EXCLUDED.install_operation_id,generation=EXCLUDED.generation,retired_through=EXCLUDED.retired_through,membership_generation=EXCLUDED.membership_generation,state='INSTALLED'`, in.SpaceID, in.BotID, in.ProfileID, in.InstallationID, in.OperationID, in.Generation, floor, generation)
	return err
}

func validateBotNativeProof(ctx context.Context, tx pgx.Tx, b *spacev1.NativeMembershipPublicationBinding) error {
	if b == nil || b.GetInitiatingService() != "bot" || b.InitiatingAccountId != nil || b.GetInitiatingProfileId() != b.GetProfileId() || b.GetProducerOperationId() != b.GetPublicationOperationId() || b.GetApplicationId() != "" || b.GetEnvironmentId() != "" || b.GetCorporationKey() != "" || b.PreviousOwnerProfileId != nil || b.PreviousOwnerAccountId != nil || b.ExpectedOwnerGeneration != nil || b.ResultOwnerGeneration != nil || b.PublishedAt == nil || b.PublishedAt.CheckValid() != nil || len(b.PublishedAt.ProtoReflect().GetUnknown()) != 0 {
		return ErrMemberAuthorityUnavailable
	}
	var in botmembershipprincipal.InstallationBinding
	var membership uint64
	var request, native []byte
	var published time.Time
	err := tx.QueryRow(ctx, `SELECT a.install_operation_id,a.space_id,a.bot_id,a.profile_id,a.installation_id,a.generation,a.membership_generation,a.add_request_sha256,a.native_publication_sha256,a.published_at FROM space_bot_installation_attempts a JOIN space_bot_installation_heads h ON h.space_id=a.space_id AND h.bot_id=a.bot_id AND h.install_operation_id=a.install_operation_id AND h.installation_id=a.installation_id AND h.profile_id=a.profile_id AND h.generation=a.generation AND h.membership_generation=a.membership_generation WHERE a.install_operation_id=$1 AND a.state='PUBLISHED' AND h.state='INSTALLED' AND h.retired_through<h.generation`, b.GetPublicationOperationId()).Scan(&in.OperationID, &in.SpaceID, &in.BotID, &in.ProfileID, &in.InstallationID, &in.Generation, &membership, &request, &native, &published)
	in.InstallOperationID = in.OperationID
	if err != nil || b.GetSpaceId() != in.SpaceID.String() || b.GetProfileId() != in.ProfileID.String() || b.GetBotId() != in.BotID.String() || b.GetInstallationId() != in.InstallationID.String() || b.GetOrdinaryGeneration() != membership || !bytes.Equal(b.GetRequestSha256(), request) || !b.PublishedAt.AsTime().Equal(published) {
		return ErrMemberAuthorityUnavailable
	}
	evidence, err := botInstallationEvidence(in, membership, request)
	if err != nil || !bytes.Equal(evidence[:], b.GetProducerEvidenceSha256()) {
		return ErrMemberAuthorityUnavailable
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(b)
	if err != nil {
		return ErrMemberAuthorityUnavailable
	}
	digest := sha256.Sum256(append([]byte(nativeMemberPublicationDomain), raw...))
	if !bytes.Equal(digest[:], native) {
		return ErrMemberAuthorityUnavailable
	}
	return nil
}

func cancelNativeBotInstallation(ctx context.Context, tx pgx.Tx, in botmembershipprincipal.InstallationBinding, requestSHA []byte) error {
	add := in
	add.Remove = false
	add.OperationID = in.InstallOperationID
	if !in.Remove || in.OperationID == uuid.Nil || in.OperationID == in.InstallOperationID {
		return ErrMemberAdmissionConflict
	}
	if _, err := botInstallationEvidence(add, 1, requestSHA); err != nil {
		return err
	}
	var savedInstall, savedSpace uuid.UUID
	var savedRequest []byte
	err := tx.QueryRow(ctx, `SELECT install_operation_id,space_id,request_sha256 FROM space_bot_installation_cancellations WHERE operation_id=$1`, in.OperationID).Scan(&savedInstall, &savedSpace, &savedRequest)
	if err == nil {
		if savedInstall != in.InstallOperationID || savedSpace != in.SpaceID || !bytes.Equal(savedRequest, requestSHA) {
			return ErrMemberAdmissionConflict
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var headGeneration, headFloor uint64
	var headOperation, headInstall uuid.UUID
	var headState string
	err = tx.QueryRow(ctx, `SELECT generation,retired_through,install_operation_id,installation_id,state FROM space_bot_installation_heads WHERE space_id=$1 AND bot_id=$2 FOR UPDATE`, in.SpaceID, in.BotID).Scan(&headGeneration, &headFloor, &headOperation, &headInstall, &headState)
	headExists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var space, bot, profile, install uuid.UUID
	var generation uint64
	var membership *uint64
	var state string
	err = tx.QueryRow(ctx, `SELECT space_id,bot_id,profile_id,installation_id,generation,membership_generation,state FROM space_bot_installation_attempts WHERE install_operation_id=$1 FOR UPDATE`, in.InstallOperationID).Scan(&space, &bot, &profile, &install, &generation, &membership, &state)
	if err == nil {
		if space != in.SpaceID || bot != in.BotID || profile != in.ProfileID || install != in.InstallationID || generation != in.Generation {
			return ErrMemberAdmissionConflict
		}
		if state == "PUBLISHED" {
			if !headExists || headOperation != in.InstallOperationID || headGeneration != in.Generation || headInstall != in.InstallationID || headState != "INSTALLED" || membership == nil {
				return ErrMemberAuthorityUnavailable
			}
			// Only the exact journal-owned incarnation may be removed. A later
			// independent source survives even if the old installation was left.
			if _, err = tx.Exec(ctx, `DELETE FROM space_members WHERE space_id=$1 AND profile_id=$2 AND membership_generation=$3 AND admission_operation_id=$4`, in.SpaceID, in.ProfileID, *membership, in.InstallOperationID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE space_bot_installation_attempts SET state='CANCELED' WHERE install_operation_id=$1`, in.InstallOperationID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE space_bot_installation_heads SET state='CANCELED',retired_through=generation WHERE space_id=$1 AND bot_id=$2 AND install_operation_id=$3`, in.SpaceID, in.BotID, in.InstallOperationID); err != nil {
				return err
			}
		} else if state != "CANCELED" {
			return ErrMemberAuthorityUnavailable
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		if (!headExists && in.Generation != 1) || (headExists && (headState != "CANCELED" || headFloor != headGeneration || headGeneration == math.MaxInt64 || in.Generation != headGeneration+1 || headInstall == in.InstallationID || headOperation == in.InstallOperationID)) {
			return ErrMemberAdmissionStale
		}
		if _, err = tx.Exec(ctx, `INSERT INTO space_bot_installation_attempts(install_operation_id,installation_id,space_id,bot_id,profile_id,generation,state) VALUES($1,$2,$3,$4,$5,$6,'CANCELED')`, in.InstallOperationID, in.InstallationID, in.SpaceID, in.BotID, in.ProfileID, in.Generation); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO space_bot_installation_heads(space_id,bot_id,profile_id,installation_id,install_operation_id,generation,retired_through,state) VALUES($1,$2,$3,$4,$5,$6,$6,'CANCELED') ON CONFLICT(space_id,bot_id) DO UPDATE SET profile_id=EXCLUDED.profile_id,installation_id=EXCLUDED.installation_id,install_operation_id=EXCLUDED.install_operation_id,generation=EXCLUDED.generation,retired_through=EXCLUDED.retired_through,membership_generation=NULL,state='CANCELED'`, in.SpaceID, in.BotID, in.ProfileID, in.InstallationID, in.InstallOperationID, in.Generation); err != nil {
			return err
		}
	} else {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO space_bot_installation_cancellations(operation_id,install_operation_id,space_id,request_sha256,completed_at) VALUES($1,$2,$3,$4,clock_timestamp())`, in.OperationID, in.InstallOperationID, in.SpaceID, requestSHA)
	return err
}
