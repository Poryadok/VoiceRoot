package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// The protected GIS adapter authenticates the exact producer request. Local
// serialization replaces an uncommitted pending/NULL row: the persisted M19
// constraints remain unchanged, and no journal changes Space identity.
func (s *SpaceStore) createNativeCommunityBootstrapSpace(ctx context.Context, in CommunityBootstrapSpaceInput, hash [32]byte) (*SpaceRow, bool, error) {
	if s.tx != nil {
		return nil, false, ErrMemberAuthorityUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, false, err
	}
	defer rollbackLifecycleTx(ctx, tx)
	// Every writer takes operation, then business identity, then Space. These
	// two-int advisory locks cannot alias the single-bigint Space fence.
	operation := append([]byte("voice.space.community-bootstrap.operation.v1\x00"), in.OperationID[:]...)
	identity := append([]byte("voice.space.community-bootstrap.identity.v1\x00"), in.ApplicationID[:]...)
	identity = append(identity, in.EnvironmentID[:]...)
	identity = append(identity, []byte(in.CorporationKey)...)
	for _, input := range [][]byte{operation, identity} {
		key := sha256.Sum256(input)
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1::integer,$2::integer)`, int32(binary.BigEndian.Uint32(key[:4])), int32(binary.BigEndian.Uint32(key[4:8]))); err != nil {
			return nil, false, err
		}
	}
	var savedHash []byte
	var status string
	var savedSpace *uuid.UUID
	err = tx.QueryRow(ctx, `SELECT request_hash,status,space_id FROM public.community_bootstrap_operations WHERE operation_id=$1 FOR UPDATE`, in.OperationID).Scan(&savedHash, &status, &savedSpace)
	if err == nil {
		if !bytes.Equal(savedHash, hash[:]) {
			return nil, false, ErrCommunityBootstrapConflict
		}
		if status != "succeeded" || savedSpace == nil {
			return nil, false, ErrMemberAuthorityUnavailable
		}
		if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{*savedSpace}); err != nil {
			return nil, false, err
		}
		if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
			return nil, false, err
		}
		row, err := scanSpaceRow(tx.QueryRow(ctx, `SELECT `+spaceSelectColumns+` FROM public.spaces WHERE id=$1`, *savedSpace))
		if err != nil {
			return nil, false, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, false, err
		}
		return row, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, false, err
	}
	var bound bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.community_bootstrap_operations WHERE application_id=$1 AND environment_id=$2 AND corporation_key=$3)`, in.ApplicationID, in.EnvironmentID, in.CorporationKey).Scan(&bound); err != nil {
		return nil, false, err
	}
	if bound {
		return nil, false, ErrCommunityBootstrapConflict
	}
	space := uuid.New()
	if err = lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{space}); err != nil {
		return nil, false, err
	}
	if err = checkMemberAuthorityCatalog(ctx, tx); err != nil {
		return nil, false, err
	}
	scoped := *s
	scoped.tx = tx
	version, err := scoped.memberDecisionSchema(ctx)
	if err != nil {
		return nil, false, err
	}
	var capture *effectiveMemberMutations
	if version == 44 || version == 45 || version == 50 {
		// Same package-private absent identity/cohort protocol as native Creator;
		// this caller's distinct kind4 proof is the exact GIS operation journal.
		capture, err = scoped.captureBornCreatorEffectiveMutation(ctx, space, in.OwnerProfileID)
		if err != nil {
			return nil, false, err
		}
	}
	row, err := scanSpaceRow(tx.QueryRow(ctx, `INSERT INTO public.spaces(id,name,description,visibility,owner_profile_id,member_count)
 VALUES($1,'Game community','Created through an approved Voice game integration','private',$2,1) RETURNING `+spaceSelectColumns, space, in.OwnerProfileID))
	if err != nil {
		return nil, false, err
	}
	if version != 44 && version != 45 && version != 50 {
		capture, err = scoped.captureNativeEffectiveMemberMutation(ctx, space, []uuid.UUID{in.OwnerProfileID})
		if err != nil {
			return nil, false, err
		}
	}
	// The succeeded journal is committed atomically with publication; there is
	// no externally visible pending Space or post-commit provenance fabrication.
	_, err = tx.Exec(ctx, `INSERT INTO public.community_bootstrap_operations
 (operation_id,application_id,environment_id,owner_account_id,owner_profile_id,corporation_key,template_id,request_hash,status,space_id)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,'succeeded',$9)`, in.OperationID, in.ApplicationID, in.EnvironmentID, in.OwnerAccountID, in.OwnerProfileID, in.CorporationKey, in.TemplateID, hash[:], space)
	if err != nil {
		if isPGUniqueViolation(err) {
			return nil, false, ErrCommunityBootstrapConflict
		}
		return nil, false, err
	}
	if err = publishCommunityBootstrapOwner(ctx, tx, space, in, hash[:]); err != nil {
		return nil, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.community_owner_authority(space_id,application_id,environment_id,corporation_key,owner_account_id,owner_profile_id,owner_generation,status)
 VALUES($1,$2,$3,$4,$5,$6,1,'active')`, space, in.ApplicationID, in.EnvironmentID, in.CorporationKey, in.OwnerAccountID, in.OwnerProfileID); err != nil {
		return nil, false, err
	}
	if capture != nil {
		if err = scoped.finishEffectiveMemberMutations(ctx, capture); err != nil {
			return nil, false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	return row, true, nil
}
