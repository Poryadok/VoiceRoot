// Package partysnapshotrpc serves only the protected Matchmaking snapshot method.
package partysnapshotrpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/partyauthority"
	"voice/backend/voice/internal/matchsquadprincipal"
	"voice/backend/voice/internal/partysnapshot"
)

type Snapshotter interface {
	Snapshot(context.Context, partysnapshot.Subject) (partysnapshot.Snapshot, error)
}

// All other Voice methods remain unimplemented on this dedicated listener.
type Server struct {
	callsv1.UnimplementedVoiceServiceServer
	Engine Snapshotter
}

func (s *Server) GetMatchmakingPartySnapshot(ctx context.Context, req *callsv1.GetMatchmakingPartySnapshotRequest) (*callsv1.GetMatchmakingPartySnapshotResponse, error) {
	if err := matchsquadprincipal.RequireSnapshot(ctx, req); err != nil {
		return nil, err
	}
	operation, err := canonicalID(req.GetSearchOperationId())
	if err != nil {
		return nil, err
	}
	account, err := canonicalID(req.GetInitiatorAccountId())
	if err != nil {
		return nil, err
	}
	profile, err := canonicalID(req.GetInitiatorProfileId())
	if err != nil {
		return nil, err
	}
	if req.GetInitiatorSessionEpoch() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "positive initiator session epoch required")
	}
	if s == nil || s.Engine == nil {
		return nil, status.Error(codes.Unavailable, "native Voice party snapshot unavailable")
	}
	subject := partysnapshot.Subject{SearchOperationID: operation, AccountID: account, ProfileID: profile, SessionEpoch: req.GetInitiatorSessionEpoch()}
	cut, err := s.Engine.Snapshot(ctx, subject)
	if err != nil {
		switch {
		case errors.Is(err, partysnapshot.ErrStaleSubject):
			return nil, subjectFailure(codes.Unauthenticated, "STALE_PARTY_SUBJECT")
		case errors.Is(err, partysnapshot.ErrIneligible):
			return nil, subjectFailure(codes.FailedPrecondition, "PARTY_SUBJECT_INELIGIBLE")
		default:
			return nil, status.Error(codes.Unavailable, "native Voice party snapshot unavailable")
		}
	}
	if !validPrivateAuthority(cut, subject) {
		return nil, status.Error(codes.Unavailable, "native Voice party snapshot unavailable")
	}
	response := &callsv1.GetMatchmakingPartySnapshotResponse{ProtocolVersion: cut.ProtocolVersion, RosterVersion: cut.RosterVersion}
	response.MemberAuthorityVersion = cut.MemberAuthorityVersion
	response.SourceAuthoritySha256 = append([]byte(nil), cut.SourceAuthorityHash[:]...)
	for _, member := range cut.MemberAuthorities {
		response.MemberAuthorities = append(response.MemberAuthorities, &callsv1.MatchmakingPartyMemberAuthority{ProfileId: member.ProfileID.String(), AccountId: member.AccountID.String(), AdmittedSessionEpoch: member.SessionEpoch})
	}
	switch cut.Kind {
	case "SOLO":
		response.Kind = callsv1.MatchmakingPartySnapshotKind_MATCHMAKING_PARTY_SNAPSHOT_KIND_SOLO
	case "VOICE_ROSTER":
		response.Kind = callsv1.MatchmakingPartySnapshotKind_MATCHMAKING_PARTY_SNAPSHOT_KIND_VOICE_ROSTER
	default:
		return nil, status.Error(codes.Unavailable, "invalid native party snapshot")
	}
	if cut.RoomID != uuid.Nil {
		response.RoomId = cut.RoomID.String()
	}
	for _, member := range cut.MemberProfileIDs {
		response.MemberProfileIds = append(response.MemberProfileIds, member.String())
	}
	return response, nil
}

func validPrivateAuthority(c partysnapshot.Snapshot, s partysnapshot.Subject) bool {
	if c.ProtocolVersion != 1 || c.MemberAuthorityVersion != partyauthority.Version || c.SourceAuthorityHash == [32]byte{} || len(c.MemberAuthorities) != len(c.MemberProfileIDs) {
		return false
	}
	if _, _, err := partyauthority.Encode(c.MemberAuthorities); err != nil {
		return false
	}
	switch c.Kind {
	case "SOLO":
		if c.RoomID != uuid.Nil || c.RosterVersion != 0 || len(c.MemberAuthorities) != 1 {
			return false
		}
	case "VOICE_ROSTER":
		if c.RoomID == uuid.Nil || c.RosterVersion == 0 || c.RosterVersion > uint64(^uint64(0)>>1) {
			return false
		}
	default:
		return false
	}
	found := false
	for i, m := range c.MemberAuthorities {
		if m.ProfileID != c.MemberProfileIDs[i] {
			return false
		}
		if m.ProfileID == s.ProfileID {
			if m.AccountID != s.AccountID || (c.Kind == "SOLO" && m.SessionEpoch != s.SessionEpoch) {
				return false
			}
			found = true
		}
	}
	return found
}

func subjectFailure(code codes.Code, reason string) error {
	s, err := status.New(code, "initiator is not eligible for a Voice party").WithDetails(&errdetails.ErrorInfo{Domain: "voice.party", Reason: reason})
	if err != nil {
		return status.Error(codes.Unavailable, "native Voice party snapshot unavailable")
	}
	return s.Err()
}

func canonicalID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, status.Error(codes.InvalidArgument, "canonical nonzero UUID required")
	}
	return id, nil
}
