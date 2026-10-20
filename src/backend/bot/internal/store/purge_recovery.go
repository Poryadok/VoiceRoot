package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	spacev1 "voice.app/voice/space/v1"
)

// A claim owns private bytes loaded from the committed P3 snapshot. Callers
// receive copies for RPC construction and cannot rebind its later SQL CAS.
type BotPurgeClaim struct {
	space, operation            uuid.UUID
	generation, claimGeneration int64
	request, requestSHA         []byte
	purge                       *commonv1.SpacePurgeRequest
	sources                     []botPurgeClaimSource
}

type botPurgeClaimSource struct {
	bot                   uuid.UUID
	raw, digest           []byte
	coverage, coverageSHA []byte
}

func (c *BotPurgeClaim) PurgeRequest() *commonv1.SpacePurgeRequest {
	if c == nil || c.purge == nil {
		return nil
	}
	return proto.Clone(c.purge).(*commonv1.SpacePurgeRequest)
}

func (c *BotPurgeClaim) Sources() []BotPurgeSource {
	if c == nil {
		return nil
	}
	result := make([]BotPurgeSource, len(c.sources))
	for i, s := range c.sources {
		_ = json.Unmarshal(s.raw, &result[i])
	}
	return result
}

func (s *BotStore) ClaimNextBotPurgeHandoff(ctx context.Context) (*BotPurgeClaim, error) {
	if s == nil || s.Pool == nil {
		return nil, ErrDeletionClaimLost
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	c := new(BotPurgeClaim)
	var count int64
	err = tx.QueryRow(ctx, `UPDATE bot_space_purge_handoffs SET claim_generation=claim_generation+1,claimed_until=clock_timestamp()+interval '30 seconds' WHERE space_id=(SELECT space_id FROM bot_space_purge_handoffs WHERE state='PENDING' AND next_attempt_at<=clock_timestamp() AND (claimed_until IS NULL OR claimed_until<=clock_timestamp()) AND claim_generation<9223372036854775807 ORDER BY next_attempt_at,space_id FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING space_id,deletion_operation_id,generation,request_bytes,request_sha256,source_count,claim_generation`).Scan(&c.space, &c.operation, &c.generation, &c.request, &c.requestSHA, &count, &c.claimGeneration)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeletionClaimLost
	}
	if err != nil {
		return nil, err
	}
	wrapper := new(botv1.PurgeSpaceRequest)
	if !purgeProofCanonical(c.request, wrapper) || !purgeProofSealed("voice.bot.v1.PurgeSpaceRequest", c.request, c.requestSHA) || wrapper.Purge == nil {
		return nil, ErrSpaceLifecycleConflict
	}
	c.purge = wrapper.Purge
	if c.purge.ProtocolVersion != 1 || c.purge.ParticipantId != commonv1.ParticipantId_PARTICIPANT_ID_BOT || c.purge.SpaceId != c.space.String() || c.purge.DeletionOperationId != c.operation.String() || c.purge.Generation != uint64(c.generation) {
		return nil, ErrSpaceLifecycleConflict
	}
	rows, err := tx.Query(ctx, `SELECT bot_id,source_bytes,source_sha256,coverage_bytes,coverage_sha256 FROM bot_space_purge_sources WHERE space_id=$1 ORDER BY bot_id`, c.space)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v botPurgeClaimSource
		if err = rows.Scan(&v.bot, &v.raw, &v.digest, &v.coverage, &v.coverageSHA); err != nil {
			break
		}
		var source BotPurgeSource
		digest := sha256.Sum256(v.raw)
		if !bytes.Equal(digest[:], v.digest) || json.Unmarshal(v.raw, &source) != nil || source.Version != 1 || source.BotID != v.bot || source.SpaceID != c.space {
			err = ErrSpaceLifecycleConflict
			break
		}
		canonical, e := json.Marshal(source)
		if e != nil || !bytes.Equal(canonical, v.raw) {
			err = ErrSpaceLifecycleConflict
			break
		}
		if (v.coverage == nil) != (v.coverageSHA == nil) {
			err = ErrSpaceLifecycleConflict
			break
		}
		if v.coverage != nil {
			sum := sha256.Sum256(v.coverage)
			if !bytes.Equal(sum[:], v.coverageSHA) {
				err = ErrSpaceLifecycleConflict
				break
			}
		}
		c.sources = append(c.sources, v)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	if count <= 0 || int64(len(c.sources)) != count {
		return nil, ErrSpaceLifecycleConflict
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// Re-enter the irreversible Space guard before row locks. Expiry and source
// equality are evaluated after ALL waits, never from a pre-wait snapshot.
func (s *BotStore) purgeClaimTx(ctx context.Context, c *BotPurgeClaim) (pgx.Tx, error) {
	if c == nil || c.space == uuid.Nil || c.operation == uuid.Nil || c.claimGeneration <= 0 {
		return nil, ErrDeletionClaimLost
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	fail := func(e error) (pgx.Tx, error) { _ = tx.Rollback(context.Background()); return nil, e }
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,71431))`, c.space.String()); err != nil {
		return fail(err)
	}
	var operation uuid.UUID
	var generation int64
	var state string
	var manifest uuid.UUID
	var manifestSHA []byte
	var itemCount int64
	err = tx.QueryRow(ctx, `SELECT deletion_operation_id,generation,desired_state,manifest_id,manifest_sha256,manifest_item_count FROM bot_space_lifecycle_heads WHERE space_id=$1 FOR UPDATE`, c.space).Scan(&operation, &generation, &state, &manifest, &manifestSHA, &itemCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrSpaceLifecycleConflict)
	}
	if err != nil {
		return fail(err)
	}
	m := c.purge.GetManifest()
	if m == nil || operation != c.operation || generation != c.generation || state != "PURGE_DECIDED" || manifest.String() != m.ManifestId || !bytes.Equal(manifestSHA, m.ManifestSha256) || itemCount != int64(m.ItemCount) {
		return fail(ErrSpaceLifecycleConflict)
	}
	for _, source := range c.sources {
		if _, err = s.GetBotByIDForUpdate(ctx, tx, source.bot); err != nil {
			return fail(err)
		}
	}
	var claimGeneration, count int64
	var raw, digest []byte
	err = tx.QueryRow(ctx, `SELECT deletion_operation_id,generation,state,request_bytes,request_sha256,source_count,claim_generation FROM bot_space_purge_handoffs WHERE space_id=$1 FOR UPDATE`, c.space).Scan(&operation, &generation, &state, &raw, &digest, &count, &claimGeneration)
	if errors.Is(err, pgx.ErrNoRows) {
		return fail(ErrDeletionClaimLost)
	}
	if err != nil {
		return fail(err)
	}
	if state != "PENDING" || claimGeneration != c.claimGeneration {
		return fail(ErrDeletionClaimLost)
	}
	if operation != c.operation || generation != c.generation || !bytes.Equal(raw, c.request) || !bytes.Equal(digest, c.requestSHA) || count != int64(len(c.sources)) {
		return fail(ErrSpaceLifecycleConflict)
	}
	for _, source := range c.sources {
		err = tx.QueryRow(ctx, `SELECT source_bytes,source_sha256 FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2 FOR UPDATE`, c.space, source.bot).Scan(&raw, &digest)
		if errors.Is(err, pgx.ErrNoRows) {
			return fail(ErrSpaceLifecycleConflict)
		}
		if err != nil {
			return fail(err)
		}
		if !bytes.Equal(raw, source.raw) || !bytes.Equal(digest, source.digest) {
			return fail(ErrSpaceLifecycleConflict)
		}
	}
	var actual int64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_sources WHERE space_id=$1`, c.space).Scan(&actual); err != nil {
		return fail(err)
	}
	if actual != count {
		return fail(ErrSpaceLifecycleConflict)
	}
	var current bool
	err = tx.QueryRow(ctx, `SELECT COALESCE(claimed_until>clock_timestamp(),false) FROM bot_space_purge_handoffs WHERE space_id=$1 AND claim_generation=$2`, c.space, c.claimGeneration).Scan(&current)
	if err != nil {
		return fail(err)
	}
	if !current {
		return fail(ErrDeletionClaimLost)
	}
	return tx, nil
}

func (s *BotStore) RetryBotPurgeHandoff(ctx context.Context, c *BotPurgeClaim) error {
	tx, err := s.purgeClaimTx(ctx, c)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	r, err := tx.Exec(ctx, `UPDATE bot_space_purge_handoffs SET claimed_until=NULL,next_attempt_at=clock_timestamp()+interval '2 seconds' WHERE space_id=$1 AND claim_generation=$2 AND claimed_until>clock_timestamp()`, c.space, c.claimGeneration)
	if err != nil {
		return err
	}
	if r.RowsAffected() != 1 {
		return ErrDeletionClaimLost
	}
	return tx.Commit(ctx)
}

// Chat batches come only from the frozen eligible source. A read request limit
// is a transport batch size, never permission to silently drop extra chats.
func (c *BotPurgeClaim) ChatBatches(bot uuid.UUID) ([][]string, error) {
	if c == nil {
		return nil, ErrDeletionClaimLost
	}
	var source *BotPurgeSource
	for _, raw := range c.sources {
		if raw.bot == bot {
			source = new(BotPurgeSource)
			if json.Unmarshal(raw.raw, source) != nil {
				return nil, ErrSpaceLifecycleConflict
			}
			break
		}
	}
	if source == nil {
		return nil, ErrSpaceLifecycleConflict
	}
	ids := map[string]struct{}{}
	add := func(id uuid.UUID) error {
		if id == uuid.Nil {
			return ErrSpaceLifecycleConflict
		}
		ids[id.String()] = struct{}{}
		return nil
	}
	for _, v := range source.ChatSources {
		if err := add(v.ChatID); err != nil {
			return nil, err
		}
	}
	if source.Intent != nil {
		for _, id := range source.Intent.Chats {
			if err := add(id); err != nil {
				return nil, err
			}
		}
	}
	for _, v := range source.Retained {
		for _, id := range v.Target.Chats {
			if err := add(id); err != nil {
				return nil, err
			}
		}
	}
	chats := make([]string, 0, len(ids))
	for id := range ids {
		chats = append(chats, id)
	}
	sort.Strings(chats)
	if len(chats) == 0 {
		return [][]string{nil}, nil
	}
	var result [][]string
	for start := 0; start < len(chats); start += 1000 {
		end := start + 1000
		if end > len(chats) {
			end = len(chats)
		}
		result = append(result, append([]string(nil), chats[start:end]...))
	}
	return result, nil
}

type purgeCoverageObservation struct {
	Kind         string                          `json:"kind"`
	Version      int                             `json:"version"`
	SourceSHA256 []byte                          `json:"source_sha256"`
	Proofs       []purgeCoverageObservationProof `json:"proofs"`
	Native       []purgeNativeObservation        `json:"native,omitempty"`
}
type purgeCoverageObservationProof struct {
	Purge, HandoffRequest, Handoff, CoverageRequest, Coverage []byte
}

func (c *BotPurgeClaim) coverageObservationBody(bot uuid.UUID, proofs []BotPurgeCoverageProof) ([]byte, error) {
	batches, err := c.ChatBatches(bot)
	if err != nil {
		return nil, err
	}
	if len(proofs) == 0 || len(proofs) > len(batches) {
		return nil, ErrSpaceLifecycleConflict
	}
	envelope := purgeCoverageObservation{Kind: "P3_COVERAGE", Version: 1}
	for _, source := range c.sources {
		if source.bot == bot {
			envelope.SourceSHA256 = append([]byte(nil), source.digest...)
		}
	}
	marshal := func(m proto.Message) ([]byte, error) {
		if m == nil || !m.ProtoReflect().IsValid() {
			return nil, nil
		}
		return (proto.MarshalOptions{Deterministic: true}).Marshal(m)
	}
	var firstHandoff *spacev1.GetBotPurgeHandoffResponse
	for i, p := range proofs {
		owned, e := ValidateBotPurgeCoverage(p, batches[i])
		if e != nil {
			return nil, e
		}
		if !proto.Equal(owned.Purge, c.purge) {
			return nil, ErrSpaceLifecycleConflict
		}
		if i == 0 {
			firstHandoff = owned.Handoff
		} else if !proto.Equal(owned.Handoff.Role, firstHandoff.Role) || !proto.Equal(owned.Handoff.Messaging, firstHandoff.Messaging) {
			return nil, ErrSpaceLifecycleConflict
		}
		v := purgeCoverageObservationProof{}
		for _, pair := range []struct {
			m proto.Message
			b *[]byte
		}{{owned.Purge, &v.Purge}, {owned.HandoffRequest, &v.HandoffRequest}, {owned.Handoff, &v.Handoff}, {owned.CoverageRequest, &v.CoverageRequest}, {owned.Coverage, &v.Coverage}} {
			*pair.b, e = marshal(pair.m)
			if e != nil {
				return nil, e
			}
		}
		envelope.Proofs = append(envelope.Proofs, v)
	}
	return json.Marshal(envelope)
}

// CoverageObservation revalidates retained typed originals and membership,
// not only a SQL state or matching hash. Callers get newly decoded objects.
func (c *BotPurgeClaim) CoverageObservation(bot uuid.UUID) ([]BotPurgeCoverageProof, error) {
	if c == nil {
		return nil, ErrDeletionClaimLost
	}
	var saved *botPurgeClaimSource
	for i := range c.sources {
		if c.sources[i].bot == bot {
			saved = &c.sources[i]
			break
		}
	}
	if saved == nil {
		return nil, ErrSpaceLifecycleConflict
	}
	if saved.coverage == nil {
		return nil, nil
	}
	var envelope purgeCoverageObservation
	if json.Unmarshal(saved.coverage, &envelope) != nil || envelope.Kind != "P3_COVERAGE" || envelope.Version != 1 || !bytes.Equal(envelope.SourceSHA256, saved.digest) {
		return nil, ErrSpaceLifecycleConflict
	}
	proofs := make([]BotPurgeCoverageProof, 0, len(envelope.Proofs))
	for _, v := range envelope.Proofs {
		p := BotPurgeCoverageProof{Purge: new(commonv1.SpacePurgeRequest), HandoffRequest: new(spacev1.GetBotPurgeHandoffRequest), Handoff: new(spacev1.GetBotPurgeHandoffResponse)}
		if !purgeProofCanonical(v.Purge, p.Purge) || !purgeProofCanonical(v.HandoffRequest, p.HandoffRequest) || !purgeProofCanonical(v.Handoff, p.Handoff) {
			return nil, ErrSpaceLifecycleConflict
		}
		if v.CoverageRequest != nil {
			p.CoverageRequest = new(messagingv1.GetBotPurgeChatCoverageRequest)
			if !purgeProofCanonical(v.CoverageRequest, p.CoverageRequest) {
				return nil, ErrSpaceLifecycleConflict
			}
		}
		if v.Coverage != nil {
			p.Coverage = new(messagingv1.GetBotPurgeChatCoverageResponse)
			if !purgeProofCanonical(v.Coverage, p.Coverage) {
				return nil, ErrSpaceLifecycleConflict
			}
		}
		proofs = append(proofs, p)
	}
	canonical, err := c.coverageObservationBody(bot, proofs)
	if err != nil {
		return nil, ErrSpaceLifecycleConflict
	}
	if err = c.validateNativeObservations(bot, envelope.Native); err != nil {
		return nil, err
	}
	var expected purgeCoverageObservation
	if json.Unmarshal(canonical, &expected) != nil {
		return nil, ErrSpaceLifecycleConflict
	}
	expected.Native = envelope.Native
	canonical, err = json.Marshal(expected)
	if err != nil || !bytes.Equal(canonical, saved.coverage) {
		return nil, ErrSpaceLifecycleConflict
	}
	return proofs, nil
}

// Save only validated, source-bound P3_COVERAGE observations. This is NOT a
// remote participant receipt or native-drain result; state stays PENDING.
func (s *BotStore) SaveBotPurgeCoverageObservation(ctx context.Context, c *BotPurgeClaim, bot uuid.UUID, proofs []BotPurgeCoverageProof) error {
	body, err := c.coverageObservationBody(bot, proofs)
	if err != nil {
		return err
	}
	tx, err := s.purgeClaimTx(ctx, c)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var saved, savedSHA []byte
	err = tx.QueryRow(ctx, `SELECT coverage_bytes,coverage_sha256 FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, c.space, bot).Scan(&saved, &savedSHA)
	if err != nil {
		return err
	}
	if (saved == nil) != (savedSHA == nil) {
		return ErrSpaceLifecycleConflict
	}
	if saved != nil {
		digest := sha256.Sum256(saved)
		if !bytes.Equal(digest[:], savedSHA) {
			return ErrSpaceLifecycleConflict
		}
	}
	if saved != nil {
		if bytes.Equal(saved, body) {
			return tx.Commit(ctx)
		}
		// Extend only the exact already accepted prefix. A bounded pass can
		// persist progress without dropping chats or replacing original proof.
		var prior, next purgeCoverageObservation
		if json.Unmarshal(saved, &prior) != nil || json.Unmarshal(body, &next) != nil || len(prior.Proofs) == 0 || len(prior.Proofs) >= len(next.Proofs) {
			return ErrSpaceLifecycleConflict
		}
		next.Proofs = next.Proofs[:len(prior.Proofs)]
		prefix, e := json.Marshal(next)
		if e != nil || !bytes.Equal(prefix, saved) {
			return ErrSpaceLifecycleConflict
		}
	}
	digest := sha256.Sum256(body)
	r, err := tx.Exec(ctx, `UPDATE bot_space_purge_sources s SET coverage_bytes=$3,coverage_sha256=$4 WHERE s.space_id=$1 AND s.bot_id=$2 AND s.state='PENDING' AND EXISTS(SELECT 1 FROM bot_space_purge_handoffs h WHERE h.space_id=s.space_id AND h.claim_generation=$5 AND h.claimed_until>clock_timestamp())`, c.space, bot, body, digest[:], c.claimGeneration)
	if err != nil {
		return err
	}
	if r.RowsAffected() != 1 {
		return ErrDeletionClaimLost
	}
	return tx.Commit(ctx)
}
