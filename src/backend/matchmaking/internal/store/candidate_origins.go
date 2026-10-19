package store

import (
	"context"
	"github.com/jackc/pgx/v5"
	admission "voice/backend/matchmaking/internal/mmadmission"
	wire "voice/backend/pkg/mmadmissionwire"
)

// candidateOriginCut is explicit candidate-v2 material. Public row JSON cannot
// silently enroll a historical source or omit current installation history.
type candidateOriginCut struct {
	Version     uint32
	Stamp       wire.AdmissionStampV1
	HistoryHash wire.Hash
	Origins     []wire.OriginV1
}

func readCandidateOrigins(ctx context.Context, tx pgx.Tx, binding *admission.Binding, parties []SourceParty) (candidateOriginCut, error) {
	return readCandidateOriginsMode(ctx, tx, binding, parties, false)
}

func readCandidateOriginsMode(ctx context.Context, tx pgx.Tx, binding *admission.Binding, parties []SourceParty, terminal bool) (candidateOriginCut, error) {
	if binding == nil {
		if err := admission.CheckPrecutCut(ctx, tx); err != nil {
			return candidateOriginCut{}, err
		}
		return candidateOriginCut{Version: 1}, nil
	}
	read := admission.ReadHistory
	if terminal {
		read = admission.ReadTerminalHistory
	}
	cut, err := read(ctx, tx, admission.WireBinding(*binding))
	if err != nil || len(parties) == 0 {
		return candidateOriginCut{}, admission.ErrUnavailable
	}
	out := candidateOriginCut{Version: 2, Stamp: cut.Stamp(), HistoryHash: cut.Hash()}
	if out.HistoryHash == (wire.Hash{}) {
		return candidateOriginCut{}, admission.ErrUnavailable
	}
	for _, party := range parties {
		origin, err := readNativeOrigin(ctx, tx, party, &cut)
		if err != nil {
			return candidateOriginCut{}, err
		}
		out.Origins = append(out.Origins, origin)
	}
	return out, nil
}
