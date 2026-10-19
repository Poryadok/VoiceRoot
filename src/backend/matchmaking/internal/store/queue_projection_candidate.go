package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	admission "voice/backend/matchmaking/internal/mmadmission"
	wire "voice/backend/pkg/mmadmissionwire"
)

// QueueProjectionCandidate is a private immutable native cut. Its source and
// admission history are distinct from Redis intent/due-time bookkeeping.
type QueueProjectionCandidate struct {
	fingerprint    [32]byte
	projectionHash [32]byte
	lease          PartyQueueLease
	party          SourceParty
	origins        candidateOriginCut
	binding        *admission.Binding
}

func (c QueueProjectionCandidate) Hash() [32]byte { return c.fingerprint }
func (c QueueProjectionCandidate) Parties() []SourceParty {
	return (ProvisionCandidate{parties: []SourceParty{c.party}}).Parties()
}
func (c QueueProjectionCandidate) Accounts() []uuid.UUID {
	return (ProposalCandidate{fingerprint: c.fingerprint, parties: []SourceParty{c.party}}).Accounts()
}

type QueueProjectionProof interface {
	Matches(QueueProjectionCandidate) bool
}

// AuthorizeQueueProjectionForDispatch returns the private witness carried into
// the eventual durable acknowledgement. The caller cannot set its stamp.
func (s *PartyStore) AuthorizeQueueProjectionForDispatch(ctx context.Context, c QueueProjectionCandidate, proof QueueProjectionProof) (PartyQueueLease, error) {
	if err := s.AuthorizeQueueProjection(ctx, c, proof); err != nil {
		return PartyQueueLease{}, err
	}
	stamp, err := wire.StampHash(c.origins.Stamp)
	if err != nil {
		return PartyQueueLease{}, ErrSourcePartyUnavailable
	}
	lease := c.lease
	lease.nativeStampHash = [32]byte(stamp)
	lease.terminalProjection = false
	return lease, nil
}

func (s *PartyStore) CaptureQueueProjection(ctx context.Context, projection PartyQueueProjection) (QueueProjectionCandidate, error) {
	if s == nil || s.Pool == nil || s.NativeAdmission == nil || !projection.Enqueue || !validPartyQueueLease(projection.Lease) || projection.Lease.Action != "ENQUEUE" || projection.Lease.projectionHash == [32]byte{} {
		return QueueProjectionCandidate{}, ErrSourcePartyUnavailable
	}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return QueueProjectionCandidate{}, ErrSourcePartyUnavailable
	}
	defer rollbackPartySearch(tx)
	c := QueueProjectionCandidate{projectionHash: projection.Lease.projectionHash, lease: projection.Lease, binding: copyNativeBinding(s.NativeAdmission)}
	c.party, err = scanSourceParty(tx.QueryRow(ctx, `SELECT `+sourcePartyCols+` FROM parties WHERE id=$1`, c.lease.PartyID))
	if err != nil {
		return QueueProjectionCandidate{}, ErrSourcePartyUnavailable
	}
	c.origins, err = readCandidateOrigins(ctx, tx, c.binding, []SourceParty{c.party})
	if err != nil {
		return QueueProjectionCandidate{}, err
	}
	actual, err := partyQueueProjectionHash(ctx, tx, c.lease.PartyID)
	if err != nil || actual != c.projectionHash {
		return QueueProjectionCandidate{}, ErrPartyQueueProjectionChanged
	}
	c.fingerprint, err = projectionCandidateHash(c)
	if err != nil || len(c.Accounts()) == 0 {
		return QueueProjectionCandidate{}, ErrSourcePartyUnavailable
	}
	if err = tx.Commit(ctx); err != nil {
		return QueueProjectionCandidate{}, ErrSourcePartyUnavailable
	}
	return c, nil
}

func projectionCandidateHash(c QueueProjectionCandidate) ([32]byte, error) {
	raw, err := json.Marshal(struct {
		LeaseToken, PartyID uuid.UUID
		Revision            int64
		Action              string
		ProjectionHash      [32]byte
		Party               SourceParty
		Origins             candidateOriginCut
		Binding             *admission.Binding
	}{c.lease.LeaseToken, c.lease.PartyID, c.lease.Revision, c.lease.Action, c.projectionHash, c.party, c.origins, c.binding})
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(append([]byte("voice.mm.queue-projection.native.v1\x00"), raw...)), nil
}

// AuthorizeQueueProjection is short and owns no network call. Closing waits for
// its FIRST local gate; the process lifetime continues through Redis and ACK.
func (s *PartyStore) AuthorizeQueueProjection(ctx context.Context, c QueueProjectionCandidate, proof QueueProjectionProof) error {
	if s == nil || s.Pool == nil || s.NativeAdmission == nil || c.binding == nil || *s.NativeAdmission != *c.binding || c.fingerprint == [32]byte{} || proof == nil || !proof.Matches(c) || len(c.Accounts()) == 0 {
		return ErrSourcePartyUnavailable
	}
	binding := copyNativeBinding(s.NativeAdmission)
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ErrSourcePartyUnavailable
	}
	defer rollbackPartySearch(tx)
	if err = admission.Acquire(ctx, tx, binding); err != nil {
		return err
	}
	ban, err := AcquirePlatformBanGuard(ctx, tx, c.Accounts())
	if err != nil {
		return err
	}
	var id uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT id FROM parties WHERE id=$1 FOR UPDATE`, c.lease.PartyID).Scan(&id); err != nil {
		return ErrSourcePartyUnavailable
	}
	rows, err := tx.Query(ctx, `SELECT id FROM search_sessions WHERE party_id=$1 ORDER BY id FOR UPDATE`, c.lease.PartyID)
	if err != nil {
		return ErrSourcePartyUnavailable
	}
	for rows.Next() {
		if rows.Scan(&id) != nil {
			rows.Close()
			return ErrSourcePartyUnavailable
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return ErrSourcePartyUnavailable
	}
	var current bool
	if err = tx.QueryRow(ctx, `SELECT true FROM matchmaking_party_search_queue_intents WHERE party_id=$1 AND revision=$2 AND lease_token=$3 AND action='ENQUEUE' AND lease_until>clock_timestamp() FOR UPDATE`, c.lease.PartyID, c.lease.Revision, c.lease.LeaseToken).Scan(&current); err != nil || !current {
		return ErrPartyQueueLeaseLost
	}
	// A separate post-wait RC statement sees bans committed while inner locks
	// were awaited. Never combine this read with lock acquisition in one CTE.
	if err = ban.Check(ctx); err != nil {
		return err
	}
	var eligible bool
	if err = tx.QueryRow(ctx, `SELECT p.disbanded_at IS NULL AND EXISTS(SELECT 1 FROM search_sessions s WHERE s.party_id=p.id) AND NOT EXISTS(SELECT 1 FROM search_sessions s WHERE s.party_id=p.id AND (s.status<>'searching' OR s.match_id IS NOT NULL OR s.timeout_at IS NULL OR s.timeout_at<=clock_timestamp())) FROM parties p WHERE p.id=$1`, c.lease.PartyID).Scan(&eligible); err != nil || !eligible {
		return ErrPartyQueueProjectionChanged
	}
	actual, err := partyQueueProjectionHash(ctx, tx, c.lease.PartyID)
	if err != nil || actual != c.projectionHash {
		return ErrPartyQueueProjectionChanged
	}
	origins, err := readCandidateOrigins(ctx, tx, binding, []SourceParty{c.party})
	if err != nil {
		return err
	}
	fresh := c
	fresh.origins = origins
	hash, err := projectionCandidateHash(fresh)
	if err != nil || hash != c.fingerprint {
		return ErrPartyQueueProjectionChanged
	}
	if err = tx.Commit(ctx); err != nil {
		return ErrSourcePartyUnavailable
	}
	return nil
}
