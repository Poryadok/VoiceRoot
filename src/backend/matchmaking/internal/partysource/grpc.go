package partysource

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"time"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

const SnapshotMethod = "/voice.calls.v1.VoiceService/GetMatchmakingPartySnapshot"

type SnapshotRPC interface {
	GetMatchmakingPartySnapshot(context.Context, *callsv1.GetMatchmakingPartySnapshotRequest, ...grpc.CallOption) (*callsv1.GetMatchmakingPartySnapshotResponse, error)
}
type ServiceIssuer interface {
	IssueService(principal.ServiceInput) (string, error)
}
type GRPCClient struct {
	Client SnapshotRPC
	Issuer ServiceIssuer
}

func (c *GRPCClient) Snapshot(ctx context.Context, s Subject) (Snapshot, error) {
	if c == nil || c.Client == nil || c.Issuer == nil || s.SearchOperationID == uuid.Nil || s.AccountID == uuid.Nil || s.ProfileID == uuid.Nil || s.SessionEpoch <= 0 {
		return Snapshot{}, ErrUnknown
	}
	req := &callsv1.GetMatchmakingPartySnapshotRequest{SearchOperationId: s.SearchOperationID.String(), InitiatorAccountId: s.AccountID.String(), InitiatorProfileId: s.ProfileID.String(), InitiatorSessionEpoch: s.SessionEpoch}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return Snapshot{}, errors.Join(ErrUnknown, err)
	}
	token, err := c.Issuer.IssueService(principal.ServiceInput{Audience: "voice", RPC: SnapshotMethod, RequestID: s.SearchOperationID.String(), RequestHash: hash})
	if err != nil || token == "" {
		return Snapshot{}, errors.Join(ErrUnknown, err)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// No delegated caller metadata is authoritative on this service-only RPC.
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", s.SearchOperationID.String()))
	resp, err := c.Client.GetMatchmakingPartySnapshot(ctx, req)
	if err != nil {
		st := status.Convert(err)
		for _, detail := range st.Details() {
			if info, ok := detail.(*errdetails.ErrorInfo); ok && info.Domain == "voice.party" && ((st.Code() == codes.Unauthenticated && info.Reason == "STALE_PARTY_SUBJECT") || (st.Code() == codes.FailedPrecondition && info.Reason == "PARTY_SUBJECT_INELIGIBLE")) {
				return Snapshot{}, ErrIneligible
			}
		}
		return Snapshot{}, errors.Join(ErrUnknown, err)
	}
	if resp == nil || len(resp.ProtoReflect().GetUnknown()) != 0 {
		return Snapshot{}, ErrUnknown
	}
	result := Snapshot{ProtocolVersion: resp.ProtocolVersion, RosterVersion: resp.RosterVersion}
	switch resp.Kind {
	case callsv1.MatchmakingPartySnapshotKind_MATCHMAKING_PARTY_SNAPSHOT_KIND_SOLO:
		result.Kind = "SOLO"
	case callsv1.MatchmakingPartySnapshotKind_MATCHMAKING_PARTY_SNAPSHOT_KIND_VOICE_ROSTER:
		result.Kind = "VOICE_ROSTER"
	default:
		return Snapshot{}, ErrUnknown
	}
	if resp.RoomId != "" {
		id, err := strictID(resp.RoomId)
		if err != nil {
			return Snapshot{}, err
		}
		result.RoomID = &id
	}
	for _, text := range resp.MemberProfileIds {
		id, err := strictID(text)
		if err != nil {
			return Snapshot{}, err
		}
		result.MemberProfileIDs = append(result.MemberProfileIDs, id)
	}
	if _, err := validateSnapshot(result, s.ProfileID); err != nil {
		return Snapshot{}, err
	}
	return result, nil
}

func strictID(text string) (uuid.UUID, error) {
	id, err := uuid.Parse(text)
	if err != nil || id == uuid.Nil || id.String() != text {
		return uuid.Nil, ErrUnknown
	}
	return id, nil
}
