package sourcecustody

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These adapters deliberately have separate pools and capabilities. Their
// callers must first qualify the database schema and actual role privileges.
type postgresProbeStore struct{ pool *pgxpool.Pool }
type postgresResponseStore struct{ pool *pgxpool.Pool }

// ReadProbe observes exactly the requested sequence for a responder already bound
// to an independently verified decision. Missing data means no new probe; a
// conflicting digest or malformed row is a failure, not a different decision.
func (s *postgresResponseStore) ReadProbe(ctx context.Context, challenge, decision [32]byte, sequence uint64) (supervisionProbe, bool, error) {
	var probe supervisionProbe
	if s == nil || s.pool == nil || ctx == nil || ctx.Err() != nil || challenge == [32]byte{} || decision == [32]byte{} || sequence == 0 || sequence > math.MaxInt64 {
		return probe, false, ErrUnavailable
	}
	var digest, nonce []byte
	err := s.pool.QueryRow(ctx, `SELECT decision_digest,nonce FROM public.user_custody_monitor_probe WHERE challenge=$1 AND sequence=$2`, challenge[:], int64(sequence)).Scan(&digest, &nonce)
	if errors.Is(err, pgx.ErrNoRows) {
		return probe, false, nil
	}
	if err != nil || !bytes.Equal(digest, decision[:]) || len(nonce) != 32 {
		return probe, false, ErrUnavailable
	}
	probe = supervisionProbe{Challenge: challenge, DecisionDigest: decision, Sequence: sequence}
	copy(probe.Nonce[:], nonce)
	if !validProbe(probe) {
		return supervisionProbe{}, false, ErrUnavailable
	}
	return probe, true, nil
}

func validProbe(p supervisionProbe) bool {
	return p.Challenge != [32]byte{} && p.DecisionDigest != [32]byte{} && p.Nonce != [32]byte{} && p.Sequence > 0 && p.Sequence <= math.MaxInt64
}

func (s *postgresProbeStore) PublishProbe(ctx context.Context, p supervisionProbe) error {
	if s == nil || s.pool == nil || ctx == nil || !validProbe(p) {
		return ErrUnavailable
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO public.user_custody_monitor_probe(challenge,sequence,decision_digest,nonce) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, p.Challenge[:], int64(p.Sequence), p.DecisionDigest[:], p.Nonce[:])
	if err != nil {
		return ErrUnavailable
	}
	var digest, nonce []byte
	if s.pool.QueryRow(ctx, `SELECT decision_digest,nonce FROM public.user_custody_monitor_probe WHERE challenge=$1 AND sequence=$2`, p.Challenge[:], int64(p.Sequence)).Scan(&digest, &nonce) != nil || !bytes.Equal(digest, p.DecisionDigest[:]) || !bytes.Equal(nonce, p.Nonce[:]) {
		return ErrUnavailable
	}
	return nil
}

func (s *postgresProbeStore) ReadResponses(ctx context.Context, current, last supervisionProbe) ([]MonitorResponse, error) {
	if s == nil || s.pool == nil || ctx == nil || (current.Sequence != 0 && !validProbe(current)) || (last.Sequence != 0 && !validProbe(last)) {
		return nil, ErrUnavailable
	}
	rows, err := s.pool.Query(ctx, `SELECT r.challenge,r.sequence,p.decision_digest,r.nonce,r.incarnation,r.kind,r.signature FROM public.user_custody_monitor_response r JOIN public.user_custody_monitor_probe p ON p.challenge=r.challenge AND p.sequence=r.sequence AND p.nonce=r.nonce WHERE (r.challenge=$1 AND r.sequence=$2) OR (r.challenge=$3 AND r.sequence=$4) LIMIT 5`, current.Challenge[:], int64(current.Sequence), last.Challenge[:], int64(last.Sequence))
	if err != nil {
		return nil, ErrUnavailable
	}
	defer rows.Close()
	var result []MonitorResponse
	for rows.Next() {
		var row MonitorResponse
		var challenge, digest, nonce []byte
		var sequence int64
		var kind int16
		if rows.Scan(&challenge, &sequence, &digest, &nonce, &row.Incarnation, &kind, &row.Signature) != nil || len(challenge) != 32 || len(digest) != 32 || len(nonce) != 32 || sequence <= 0 || kind < 1 || kind > 3 || len(row.Signature) != ed25519.SignatureSize {
			return nil, ErrUnavailable
		}
		copy(row.Challenge[:], challenge)
		copy(row.DecisionDigest[:], digest)
		copy(row.Nonce[:], nonce)
		row.Sequence, row.Kind = uint64(sequence), MonitorResponseKind(kind)
		probe := supervisionProbe{Challenge: row.Challenge, DecisionDigest: row.DecisionDigest, Sequence: row.Sequence, Nonce: row.Nonce}
		if (probe != current && probe != last) || (row.Kind == MonitorArmed && row.Sequence != 1) || (row.Kind == MonitorHeartbeat && row.Sequence == 1) {
			return nil, ErrUnavailable
		}
		if _, err := MonitorResponsePayload(row); err != nil {
			return nil, ErrUnavailable
		}
		result = append(result, row)
		if len(result) > 4 {
			return nil, ErrUnavailable
		}
	}
	if rows.Err() != nil {
		return nil, ErrUnavailable
	}
	return result, nil
}

func (s *postgresResponseStore) InsertExactResponse(ctx context.Context, r MonitorResponse) error {
	if s == nil || s.pool == nil || ctx == nil || len(r.Signature) != ed25519.SignatureSize {
		return ErrUnavailable
	}
	if _, err := MonitorResponsePayload(r); err != nil || (r.Kind == MonitorArmed && r.Sequence != 1) || (r.Kind == MonitorHeartbeat && r.Sequence == 1) {
		return ErrUnavailable
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO public.user_custody_monitor_response(challenge,sequence,kind,nonce,incarnation,signature) SELECT challenge,sequence,$3,nonce,$5,$6 FROM public.user_custody_monitor_probe WHERE challenge=$1 AND sequence=$2 AND nonce=$4 AND decision_digest=$7 ON CONFLICT DO NOTHING`, r.Challenge[:], int64(r.Sequence), int16(r.Kind), r.Nonce[:], r.Incarnation, r.Signature, r.DecisionDigest[:])
	if err != nil {
		return ErrUnavailable
	}
	var nonce, signature, digest []byte
	var incarnation = r.Incarnation
	if s.pool.QueryRow(ctx, `SELECT r.nonce,r.incarnation,r.signature,p.decision_digest FROM public.user_custody_monitor_response r JOIN public.user_custody_monitor_probe p ON p.challenge=r.challenge AND p.sequence=r.sequence AND p.nonce=r.nonce WHERE r.challenge=$1 AND r.sequence=$2 AND r.kind=$3`, r.Challenge[:], int64(r.Sequence), int16(r.Kind)).Scan(&nonce, &incarnation, &signature, &digest) != nil || !bytes.Equal(nonce, r.Nonce[:]) || incarnation != r.Incarnation || !bytes.Equal(signature, r.Signature) || !bytes.Equal(digest, r.DecisionDigest[:]) {
		return ErrUnavailable
	}
	return nil
}
