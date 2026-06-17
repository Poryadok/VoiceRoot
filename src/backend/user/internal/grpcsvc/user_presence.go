package grpcsvc

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"
	"voice/backend/pkg/guestguard"

	userv1 "voice.app/voice/user/v1"
)

func (s *UserGRPC) UpdatePresence(ctx context.Context, req *userv1.UpdatePresenceRequest) (*userv1.UpdatePresenceResponse, error) {
	if s.Presence == nil {
		return nil, status.Error(codes.Unavailable, "presence store not configured")
	}
	accountID, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	profileID, err := s.resolveOwnedActiveProfile(ctx, accountID)
	if err != nil {
		return nil, err
	}
	st, enum, err := normalizePresenceInput(req)
	if err != nil {
		return nil, err
	}
	in := store.PresenceUpsert{
		Status:       st,
		StatusEnum:   enum,
		GameTitle:    req.GetGameTitle(),
		CustomStatus: req.GetCustomStatus(),
		CallInfoJSON: req.GetCallInfoJson(),
		Now:          time.Now().UTC(),
	}
	if err := s.Presence.Upsert(ctx, profileID, in); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &userv1.UpdatePresenceResponse{}, nil
}

func (s *UserGRPC) GetPresence(ctx context.Context, req *userv1.GetPresenceRequest) (*userv1.GetPresenceResponse, error) {
	if s.Presence == nil {
		return nil, status.Error(codes.Unavailable, "presence store not configured")
	}
	profileID, err := uuid.Parse(strings.TrimSpace(req.GetProfileId()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid profile_id")
	}
	snap, err := s.Presence.Get(ctx, profileID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if snap != nil && snap.Live && !s.guestMayViewOnlineStatus(ctx, profileID) {
		return &userv1.GetPresenceResponse{PresenceStatus: presenceSnapshotToProto(profileID, nil)}, nil
	}
	return &userv1.GetPresenceResponse{PresenceStatus: presenceSnapshotToProto(profileID, snap)}, nil
}

func (s *UserGRPC) guestMayViewOnlineStatus(ctx context.Context, targetProfile uuid.UUID) bool {
	if !guestguard.IsGuest(ctx) {
		return true
	}
	privacyStore := s.privacyStore()
	if privacyStore == nil {
		return false
	}
	privacy, err := privacyStore.GetByProfileID(ctx, targetProfile)
	if err != nil || privacy == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(privacy.ShowOnline), "everyone") {
		return true
	}
	return privacy.ShowOnlineIncludeGuests
}

func (s *UserGRPC) GetBulkPresence(ctx context.Context, req *userv1.GetBulkPresenceRequest) (*userv1.GetBulkPresenceResponse, error) {
	if s.Presence == nil {
		return nil, status.Error(codes.Unavailable, "presence store not configured")
	}
	raw := req.GetProfileIds()
	if len(raw) == 0 {
		return &userv1.GetBulkPresenceResponse{ByProfileId: map[string]*userv1.PresenceStatus{}}, nil
	}
	ids := make([]uuid.UUID, 0, len(raw))
	for _, sid := range raw {
		id, err := uuid.Parse(strings.TrimSpace(sid))
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid profile_id in batch")
		}
		ids = append(ids, id)
	}
	m, err := s.Presence.GetMany(ctx, ids)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make(map[string]*userv1.PresenceStatus, len(m))
	for id, snap := range m {
		out[id.String()] = presenceSnapshotToProto(id, snap)
	}
	return &userv1.GetBulkPresenceResponse{ByProfileId: out}, nil
}

func (s *UserGRPC) resolveOwnedActiveProfile(ctx context.Context, accountID uuid.UUID) (uuid.UUID, error) {
	if pid, ok := authctx.ProfileID(ctx); ok {
		row, err := s.Profiles.GetByID(ctx, pid)
		if err != nil {
			return uuid.Nil, status.Error(codes.Internal, err.Error())
		}
		if row == nil || row.AccountID != accountID {
			return uuid.Nil, status.Error(codes.NotFound, "profile not found or not owned")
		}
		return pid, nil
	}
	pid, err := s.Profiles.GetPrimaryProfileIDForAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, status.Error(codes.NotFound, "profile not found")
		}
		return uuid.Nil, status.Error(codes.Internal, err.Error())
	}
	return pid, nil
}

func normalizePresenceInput(req *userv1.UpdatePresenceRequest) (canonStatus string, statusEnum int32, err error) {
	if req.GetStatusEnum() != userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_UNSPECIFIED {
		e := req.GetStatusEnum()
		return presenceEnumToCanonicalString(e), int32(e), nil
	}
	st := strings.TrimSpace(strings.ToLower(req.GetStatus()))
	if st == "" {
		return "", 0, status.Error(codes.InvalidArgument, "status or status_enum required")
	}
	e, ok := canonicalPresenceStringToEnum(st)
	if !ok {
		return "", 0, status.Error(codes.InvalidArgument, "invalid status")
	}
	return st, int32(e), nil
}

func presenceEnumToCanonicalString(e userv1.PresenceOnlineStatus) string {
	switch e {
	case userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE:
		return "online"
	case userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_IDLE:
		return "idle"
	case userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND:
		return "dnd"
	case userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_INVISIBLE:
		return "invisible"
	default:
		return ""
	}
}

func canonicalPresenceStringToEnum(s string) (userv1.PresenceOnlineStatus, bool) {
	switch s {
	case "online":
		return userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_ONLINE, true
	case "idle":
		return userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_IDLE, true
	case "dnd":
		return userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_DND, true
	case "invisible":
		return userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_INVISIBLE, true
	default:
		return userv1.PresenceOnlineStatus_PRESENCE_ONLINE_STATUS_UNSPECIFIED, false
	}
}

func presenceSnapshotToProto(profileID uuid.UUID, snap *store.PresenceSnapshot) *userv1.PresenceStatus {
	out := &userv1.PresenceStatus{ProfileId: profileID.String()}
	if snap == nil {
		return out
	}
	if snap.Live {
		out.Status = snap.Status
		if snap.StatusEnum != 0 {
			e := userv1.PresenceOnlineStatus(snap.StatusEnum)
			out.StatusEnum = &e
		}
		if snap.GameTitle != "" {
			out.GameTitle = proto.String(snap.GameTitle)
		}
		if snap.CustomStatus != "" {
			out.CustomStatus = proto.String(snap.CustomStatus)
		}
		if snap.CallInfoJSON != "" {
			out.CallInfoJson = proto.String(snap.CallInfoJSON)
		}
		if snap.LastSeenUnix > 0 {
			out.LastSeen = timestamppb.New(time.Unix(snap.LastSeenUnix, 0).UTC())
		}
		return out
	}
	if snap.LastSeenUnix > 0 {
		out.LastSeen = timestamppb.New(time.Unix(snap.LastSeenUnix, 0).UTC())
	}
	return out
}
