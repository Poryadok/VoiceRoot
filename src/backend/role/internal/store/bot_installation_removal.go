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
	rolev1 "voice.app/voice/role/v1"
)

var (
	ErrBotRemovalInvalid  = errors.New("invalid Bot removal request")
	ErrBotRemovalConflict = errors.New("Bot removal operation conflicts")
	ErrBotRemovalDenied   = errors.New("Bot removal binding denied")
	ErrBotRemovalBlocked  = errors.New("Bot has forbidden Owner authority")
)

type botRemovalBinding struct {
	operation, decision, bot, actor, space, installation, installOperation uuid.UUID
	evidenceKind                                                           string
	generation                                                             int64
	requestBytes, evidenceBytes                                            []byte
	requestHash, evidenceHash                                              [32]byte
}

func botRemovalUUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || id.String() != raw {
		return uuid.Nil, ErrBotRemovalInvalid
	}
	return id, nil
}

func validateBotRemoval(req *rolev1.ApplyBotInstallationRemovalRequest) (b botRemovalBinding, err error) {
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.ProtocolVersion != 1 || len(req.DecisionSha256) != 32 || len(req.FrozenScopeSha256) != 32 {
		return b, ErrBotRemovalInvalid
	}
	// Canonicalize each identity independently, even when values coincide.
	b.operation, err = botRemovalUUID(req.OperationId)
	if err != nil {
		return b, err
	}
	b.decision, err = botRemovalUUID(req.DecisionId)
	if err != nil {
		return b, err
	}
	b.bot, err = botRemovalUUID(req.BotId)
	if err != nil {
		return b, err
	}
	b.actor, err = botRemovalUUID(req.ActorProfileId)
	if err != nil {
		return b, err
	}
	b.space, err = botRemovalUUID(req.SpaceId)
	if err != nil {
		return b, err
	}
	if req.Purpose != "global_delete" && req.Purpose != "space_uninstall" {
		return b, ErrBotRemovalDenied
	}
	var evidence proto.Message
	switch e := req.Evidence.(type) {
	case *rolev1.ApplyBotInstallationRemovalRequest_Native:
		if e.Native == nil || e.Native.Generation == 0 || e.Native.Generation > math.MaxInt64 {
			return b, ErrBotRemovalInvalid
		}
		b.evidenceKind = "native"
		evidence = e.Native
		b.generation = int64(e.Native.Generation)
		b.installation, err = botRemovalUUID(e.Native.InstallationId)
		if err != nil {
			return b, err
		}
		b.installOperation, err = botRemovalUUID(e.Native.InstallOperationId)
		if err != nil {
			return b, err
		}
	case *rolev1.ApplyBotInstallationRemovalRequest_LegacyInstallation:
		if e.LegacyInstallation == nil || len(e.LegacyInstallation.SnapshotSha256) != 32 {
			return b, ErrBotRemovalInvalid
		}
		b.evidenceKind = "legacy_installation"
		evidence = e.LegacyInstallation
		b.installation, err = botRemovalUUID(e.LegacyInstallation.InstallationId)
		if err != nil {
			return b, err
		}
		if _, err = botRemovalUUID(e.LegacyInstallation.SnapshotId); err != nil {
			return b, err
		}
	case *rolev1.ApplyBotInstallationRemovalRequest_OrphanWhitelist:
		if e.OrphanWhitelist == nil || len(e.OrphanWhitelist.SnapshotSha256) != 32 {
			return b, ErrBotRemovalInvalid
		}
		if req.Purpose != "global_delete" {
			return b, ErrBotRemovalDenied
		}
		b.evidenceKind = "orphan_whitelist"
		evidence = e.OrphanWhitelist
		if _, err = botRemovalUUID(e.OrphanWhitelist.SnapshotId); err != nil {
			return b, err
		}
	default:
		return b, ErrBotRemovalInvalid
	}
	if len(evidence.ProtoReflect().GetUnknown()) != 0 {
		return b, ErrBotRemovalInvalid
	}
	b.requestBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(req)
	if err != nil {
		return b, err
	}
	b.evidenceBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(evidence)
	if err != nil {
		return b, err
	}
	b.requestHash = sha256.Sum256(b.requestBytes)
	b.evidenceHash = sha256.Sum256(b.evidenceBytes)
	return b, nil
}

// ValidateBotInstallationRemoval rejects malformed source evidence before any
// store access. Authentication is repeated by the private handler.
func ValidateBotInstallationRemoval(req *rolev1.ApplyBotInstallationRemovalRequest) error {
	_, err := validateBotRemoval(req)
	return err
}

func (s *RoleStore) lockBotRemovalBinding(ctx context.Context, bot, actor uuid.UUID) error {
	if _, err := s.db().Exec(ctx, "SELECT pg_advisory_xact_lock(1380074034,hashtext($1))", bot.String()); err != nil {
		return err
	}
	if _, err := s.db().Exec(ctx, "SELECT pg_advisory_xact_lock(1380074035,hashtext($1))", actor.String()); err != nil {
		return err
	}
	// Existing actor identity cannot be rebound, including from another bot.
	var knownBot, knownActor uuid.UUID
	err := s.db().QueryRow(ctx, "SELECT bot_id,actor_profile_id FROM role_bot_actor_bindings WHERE bot_id=$1 OR actor_profile_id=$2 ORDER BY bot_id LIMIT 1", bot, actor).Scan(&knownBot, &knownActor)
	if err == nil {
		if knownBot != bot || knownActor != actor {
			return ErrBotRemovalDenied
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = s.db().Exec(ctx, "INSERT INTO role_bot_actor_bindings(bot_id,actor_profile_id) VALUES($1,$2)", bot, actor)
	return err
}

// CheckBotInstallationRemovalFence MUST run on the same WithinSpaces store as
// the admitted bot_actor mutation. It deliberately rejects unscoped callers.
// The verified capability already owns these identities; headers are not input.
func (s *RoleStore) CheckBotInstallationRemovalFence(ctx context.Context, bot, actor, space, installation uuid.UUID) error {
	if s == nil || s.tx == nil || bot == uuid.Nil || actor == uuid.Nil || installation == uuid.Nil {
		return ErrScopeUnavailable
	}
	if _, ok := s.scopeSpaces[space]; !ok {
		return ErrScopeUnavailable
	}
	if err := s.lockBotRemovalBinding(ctx, bot, actor); err != nil {
		return err
	}
	var knownBot, knownActor, knownSpace uuid.UUID
	err := s.db().QueryRow(ctx, "SELECT bot_id,actor_profile_id,space_id FROM role_bot_installation_removals WHERE installation_id=$1", installation).Scan(&knownBot, &knownActor, &knownSpace)
	if err == nil && (knownBot != bot || knownActor != actor || knownSpace != space) {
		return ErrBotRemovalDenied
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var removed bool
	if err := s.db().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_bot_global_removals WHERE bot_id=$1) OR EXISTS(SELECT 1 FROM role_bot_installation_removals WHERE bot_id=$1 AND space_id=$2 AND installation_id=$3)`, bot, space, installation).Scan(&removed); err != nil {
		return err
	}
	if removed {
		return ErrBotRemovalDenied
	}
	return nil
}

// ApplyBotInstallationRemoval commits the safety fences, scoped effects and
// immutable response together. A forbidden Owner anomaly instead commits only
// the safety fence and exact blocked obligation, then returns no receipt.
func (s *RoleStore) ApplyBotInstallationRemoval(ctx context.Context, req *rolev1.ApplyBotInstallationRemovalRequest) (*rolev1.ApplyBotInstallationRemovalResponse, error) {
	if req == nil {
		return nil, ErrBotRemovalInvalid
	}
	req = proto.Clone(req).(*rolev1.ApplyBotInstallationRemovalRequest)
	b, err := validateBotRemoval(req)
	if err != nil {
		return nil, err
	}
	var response *rolev1.ApplyBotInstallationRemovalResponse
	blocked := false
	err = s.withLockedTransaction(ctx, b.operation, []uuid.UUID{b.space}, func(scoped *RoleStore) error {
		if err := scoped.lockBotRemovalBinding(ctx, b.bot, b.actor); err != nil {
			return err
		}
		var savedRequest, savedResponse []byte
		err := scoped.db().QueryRow(ctx, "SELECT request_bytes,response_bytes FROM role_bot_removal_operations WHERE operation_id=$1", b.operation).Scan(&savedRequest, &savedResponse)
		exists := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if exists {
			if !bytes.Equal(savedRequest, b.requestBytes) {
				return ErrBotRemovalConflict
			}
			if savedResponse != nil {
				response = &rolev1.ApplyBotInstallationRemovalResponse{}
				return proto.Unmarshal(savedResponse, response)
			}
		}
		return scoped.WithinSpaces(ctx, []uuid.UUID{b.space}, func(scoped *RoleStore) error {
			db := scoped.db()
			var decision uuid.UUID
			var decisionHash []byte
			err := db.QueryRow(ctx, "SELECT decision_id,decision_sha256 FROM role_bot_global_removals WHERE bot_id=$1", b.bot).Scan(&decision, &decisionHash)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if err == nil && (req.Purpose != "global_delete" || decision != b.decision || !bytes.Equal(decisionHash, req.DecisionSha256)) {
				return ErrBotRemovalDenied
			}
			if req.Purpose == "global_delete" && errors.Is(err, pgx.ErrNoRows) {
				if _, err = db.Exec(ctx, "INSERT INTO role_bot_global_removals(bot_id,actor_profile_id,decision_id,decision_sha256) VALUES($1,$2,$3,$4)", b.bot, b.actor, b.decision, req.DecisionSha256); err != nil {
					return err
				}
			}
			var installation any
			if b.installation != uuid.Nil {
				installation = b.installation
				if _, err = db.Exec(ctx, "SELECT pg_advisory_xact_lock(1380074036,hashtext($1))", b.installation.String()); err != nil {
					return err
				}
				var actor, knownBot, knownSpace uuid.UUID
				var kind string
				var add *uuid.UUID
				var generation *int64
				err = db.QueryRow(ctx, "SELECT actor_profile_id,bot_id,space_id,evidence_kind,install_operation_id,generation FROM role_bot_installation_removals WHERE installation_id=$1", b.installation).Scan(&actor, &knownBot, &knownSpace, &kind, &add, &generation)
				if err == nil {
					if actor != b.actor || knownBot != b.bot || knownSpace != b.space || kind != b.evidenceKind || (kind == "native" && (add == nil || generation == nil || *add != b.installOperation || *generation != b.generation)) {
						return ErrBotRemovalDenied
					}
				} else if errors.Is(err, pgx.ErrNoRows) {
					var addArg, generationArg any
					if b.evidenceKind == "native" {
						addArg = b.installOperation
						generationArg = b.generation
					}
					if _, err = db.Exec(ctx, "INSERT INTO role_bot_installation_removals(bot_id,space_id,installation_id,actor_profile_id,evidence_kind,install_operation_id,generation) VALUES($1,$2,$3,$4,$5,$6,$7)", b.bot, b.space, b.installation, b.actor, b.evidenceKind, addArg, generationArg); err != nil {
						return err
					}
				} else {
					return err
				}
			}
			if !exists {
				if _, err = db.Exec(ctx, "INSERT INTO role_bot_removal_operations(operation_id,bot_id,actor_profile_id,space_id,installation_id,request_bytes,request_sha256) VALUES($1,$2,$3,$4,$5,$6,$7)", b.operation, b.bot, b.actor, b.space, installation, b.requestBytes, b.requestHash[:]); err != nil {
					return err
				}
			}
			var owner bool
			if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM member_roles m JOIN roles r ON r.id=m.role_id WHERE m.space_id=$1 AND m.profile_id=$2 AND r.name='Owner') OR EXISTS(SELECT 1 FROM role_member_owner_authority WHERE space_id=$1 AND profile_id=$2) OR EXISTS(SELECT 1 FROM role_member_contributions c JOIN roles r ON r.id=c.role_id WHERE c.space_id=$1 AND c.profile_id=$2 AND r.name='Owner')`, b.space, b.actor).Scan(&owner); err != nil {
				return err
			}
			if owner {
				blocked = true
				return nil
			}
			// A terminal result for this exact old installation is adopted, never
			// rerun against state created by a subsequent legitimate installation.
			var completed bool
			if b.installation != uuid.Nil {
				if err = db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM role_bot_removal_operations WHERE bot_id=$1 AND space_id=$2 AND installation_id=$3 AND response_bytes IS NOT NULL)", b.bot, b.space, b.installation).Scan(&completed); err != nil {
					return err
				}
			}
			var deleted, assignments int64
			if !completed {
				// Materialize exact custom role IDs before deleting typed references;
				// historical admission receipts intentionally have no live-role FK.
				rows, err := db.Query(ctx, "SELECT id FROM roles WHERE space_id=$1 AND created_by_profile_id=$2 AND NOT is_system AND name<>'Owner' ORDER BY id FOR UPDATE", b.space, b.actor)
				if err != nil {
					return err
				}
				ids := []uuid.UUID{}
				for rows.Next() {
					var id uuid.UUID
					if err = rows.Scan(&id); err != nil {
						rows.Close()
						return err
					}
					ids = append(ids, id)
				}
				err = rows.Err()
				rows.Close()
				if err != nil {
					return err
				}
				tag, err := db.Exec(ctx, `DELETE FROM role_member_contributions c USING roles r WHERE c.role_id=r.id AND c.space_id=$1 AND r.space_id=$1 AND NOT r.is_system AND r.name<>'Owner' AND (c.profile_id=$2 OR r.id=ANY($3))`, b.space, b.actor, ids)
				if err != nil {
					return err
				}
				assignments += tag.RowsAffected()
				if _, err = db.Exec(ctx, `DELETE FROM role_member_admission_preparations p USING roles r WHERE p.role_id=r.id AND p.space_id=$1 AND r.space_id=$1 AND NOT r.is_system AND r.name<>'Owner' AND (p.profile_id=$2 OR r.id=ANY($3))`, b.space, b.actor, ids); err != nil {
					return err
				}
				tag, err = db.Exec(ctx, `DELETE FROM member_roles m USING roles r WHERE m.role_id=r.id AND m.space_id=$1 AND r.space_id=$1 AND NOT r.is_system AND r.name<>'Owner' AND (m.profile_id=$2 OR r.id=ANY($3))`, b.space, b.actor, ids)
				if err != nil {
					return err
				}
				assignments += tag.RowsAffected()
				tag, err = db.Exec(ctx, "DELETE FROM roles WHERE space_id=$1 AND id=ANY($2) AND NOT is_system AND name<>'Owner'", b.space, ids)
				if err != nil {
					return err
				}
				deleted = tag.RowsAffected()
			}
			var now time.Time
			if err = db.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
				return err
			}
			response = &rolev1.ApplyBotInstallationRemovalResponse{Receipt: &rolev1.BotInstallationRemovalReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: req.OperationId, DecisionId: req.DecisionId, Purpose: req.Purpose, BotId: req.BotId, ActorProfileId: req.ActorProfileId, SpaceId: req.SpaceId, EvidenceKind: b.evidenceKind, EvidenceSha256: b.evidenceHash[:], FrozenScopeSha256: bytes.Clone(req.FrozenScopeSha256), RequestSha256: b.requestHash[:], Participant: "role", Outcome: "COMPLETED", CompletedAt: timestamppb.New(now.UTC()), DeletedRoleCount: uint64(deleted), RemovedAssignmentCount: uint64(assignments)}}
			wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
			if err != nil {
				return err
			}
			_, err = db.Exec(ctx, "UPDATE role_bot_removal_operations SET response_bytes=$2 WHERE operation_id=$1 AND response_bytes IS NULL", b.operation, wire)
			return err
		})
	})
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrBotRemovalBlocked
	}
	return response, nil
}
