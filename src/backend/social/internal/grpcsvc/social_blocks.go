package grpcsvc

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/social/internal/authctx"
	"voice/backend/social/internal/store"

	socialv1 "voice.app/voice/social/v1"
)

const maxProfilePairBlockBatch = 500

const (
	blocksDefaultPage = 20
	blocksMaxPage     = 50
)

type blocksCursorPayload struct {
	CreatedAtUnixNano int64  `json:"ts"`
	ID                string `json:"id"`
}

func encodeBlocksCursor(createdAt time.Time, id uuid.UUID) (string, error) {
	p := blocksCursorPayload{CreatedAtUnixNano: createdAt.UnixNano(), ID: id.String()}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return "b1." + base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeBlocksCursor(s string) (*store.BlocksListCursor, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if !strings.HasPrefix(s, "b1.") {
		return nil, errors.New("invalid blocks cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(s, "b1."))
	if err != nil {
		return nil, errors.New("invalid blocks cursor")
	}
	var p blocksCursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errors.New("invalid blocks cursor")
	}
	id, err := uuid.Parse(p.ID)
	if err != nil {
		return nil, errors.New("invalid blocks cursor")
	}
	ts := time.Unix(0, p.CreatedAtUnixNano).UTC()
	return &store.BlocksListCursor{CreatedAt: ts, ID: id}, nil
}

// BlockAccount implements voice.social.v1.SocialService.
func (s *SocialGRPC) BlockAccount(ctx context.Context, req *socialv1.BlockAccountRequest) (*socialv1.BlockAccountResponse, error) {
	blocker, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	blocked, err := parseUUIDField("blocked_account_id", req.GetBlockedAccountId())
	if err != nil {
		return nil, err
	}
	if s.Blocks == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	if blocker == blocked {
		return nil, status.Error(codes.InvalidArgument, store.ErrSelfBlock.Error())
	}
	if s.AccountProfiles == nil {
		return nil, status.Error(codes.FailedPrecondition, "account profile resolution not configured")
	}
	blockerProfiles, err := resolveAccountProfiles(ctx, s.AccountProfiles, blocker)
	if err != nil {
		return nil, err
	}
	blockedProfiles, err := resolveAccountProfiles(ctx, s.AccountProfiles, blocked)
	if err != nil {
		return nil, err
	}
	var selectedProfileID uuid.UUID
	var selectedProfile *BlockedProfile
	if req.GetBlockedProfileId() != "" {
		selectedProfileID, err = parseUUIDField("blocked_profile_id", req.GetBlockedProfileId())
		if err != nil {
			return nil, err
		}
		if s.BlockedProfiles == nil {
			return nil, status.Error(codes.FailedPrecondition, "blocked profile resolution not configured")
		}
		profile, lookupErr := s.BlockedProfiles.Profile(ctx, selectedProfileID)
		if lookupErr != nil {
			if st, ok := status.FromError(lookupErr); ok {
				return nil, status.Error(st.Code(), "blocked profile lookup failed")
			}
			return nil, status.Error(codes.Unavailable, "blocked profile lookup unavailable")
		}
		if profile.AccountID == uuid.Nil {
			return nil, status.Error(codes.FailedPrecondition, "blocked profile has invalid account")
		}
		if profile.AccountID != blocked {
			return nil, status.Error(codes.InvalidArgument, "blocked_profile_id does not belong to blocked_account_id")
		}
		selectedProfile = &profile
	}
	err = s.Blocks.BlockAccountAndSeverFriendshipsWithProfile(ctx, blocker, blocked, blockerProfiles, blockedProfiles, selectedProfileID, selectedProfile)
	switch {
	case err == nil:
		if s.Events != nil {
			_ = s.Events.PublishUserBlocked(ctx, blocker.String(), blocked.String())
		}
		return &socialv1.BlockAccountResponse{}, nil
	case errors.Is(err, store.ErrSelfBlock):
		return nil, status.Error(codes.InvalidArgument, err.Error())
	default:
		return nil, status.Error(codes.Internal, err.Error())
	}
}

func resolveAccountProfiles(ctx context.Context, resolver AccountProfilesResolver, accountID uuid.UUID) ([]uuid.UUID, error) {
	ids, err := resolver.ProfileIDsForAccount(ctx, accountID)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "account profile resolution unavailable")
	}
	if len(ids) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "account profile resolution returned no profiles")
	}
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, status.Error(codes.FailedPrecondition, "account profile resolution returned invalid profile")
		}
	}
	return ids, nil
}

// UnblockAccount implements voice.social.v1.SocialService.
func (s *SocialGRPC) UnblockAccount(ctx context.Context, req *socialv1.UnblockAccountRequest) (*socialv1.UnblockAccountResponse, error) {
	blocker, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	blocked, err := parseUUIDField("blocked_account_id", req.GetBlockedAccountId())
	if err != nil {
		return nil, err
	}
	if s.Blocks == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	err = s.Blocks.UnblockAccount(ctx, blocker, blocked)
	if err != nil {
		if errors.Is(err, store.ErrBlockNotFound) {
			return nil, status.Error(codes.NotFound, "block not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &socialv1.UnblockAccountResponse{}, nil
}

// ListBlocked implements voice.social.v1.SocialService.
func (s *SocialGRPC) ListBlocked(ctx context.Context, req *socialv1.ListBlockedRequest) (*socialv1.ListBlockedResponse, error) {
	blocker, ok := authctx.AccountID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	if s.Blocks == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	page := req.GetPage()
	pageSize := blocksDefaultPage
	if page != nil && page.GetPageSize() > 0 {
		pageSize = int(page.GetPageSize())
	}
	if pageSize > blocksMaxPage {
		pageSize = blocksMaxPage
	}
	cursorIn := ""
	if page != nil {
		cursorIn = page.GetCursor()
	}
	after, err := decodeBlocksCursor(cursorIn)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	rows, err := s.Blocks.ListBlocked(ctx, blocker, after, pageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}
	out := make([]*socialv1.BlockedAccount, 0, len(rows))
	var next string
	for i, r := range rows {
		profileID := ""
		if r.BlockedProfileID != uuid.Nil {
			profileID = r.BlockedProfileID.String()
		}
		out = append(out, &socialv1.BlockedAccount{
			BlockedAccountId: r.BlockedAccountID.String(),
			BlockedProfileId: profileID,
			DisplayName:      r.DisplayName,
			Username:         r.Username,
			Discriminator:    r.Discriminator,
			CreatedAt:        timestamppb.New(r.CreatedAt.UTC()),
		})
		if hasMore && i == len(rows)-1 {
			next, err = encodeBlocksCursor(r.CreatedAt, r.BlockID)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}
	return &socialv1.ListBlockedResponse{
		BlockedList: &socialv1.BlockedList{
			Blocked:    out,
			NextCursor: next,
		},
	}, nil
}

// IsBlocked implements voice.social.v1.SocialService (internal S2S: Chat/Messaging/User).
// Returns whether account_id_a has blocked account_id_b (ordered); callers check both directions for mutual exclusion.
// Does not require end-user gRPC metadata.
func (s *SocialGRPC) IsBlocked(ctx context.Context, req *socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error) {
	blocker, err := parseUUIDField("account_id_a", req.GetAccountIdA())
	if err != nil {
		return nil, err
	}
	blocked, err := parseUUIDField("account_id_b", req.GetAccountIdB())
	if err != nil {
		return nil, err
	}
	if s.Blocks == nil {
		return nil, status.Error(codes.FailedPrecondition, "persistence not configured")
	}
	if blocker == blocked {
		return &socialv1.IsBlockedResponse{Blocked: false}, nil
	}
	ok, err := s.Blocks.DirectedBlockExists(ctx, blocker, blocked)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &socialv1.IsBlockedResponse{Blocked: ok}, nil
}

// IsProfilePairBlocked is an internal directional visibility check for profile
// pairs. Account identifiers remain inside Social; callers receive only a bool.
func (s *SocialGRPC) IsProfilePairBlocked(ctx context.Context, req *socialv1.IsProfilePairBlockedRequest) (*socialv1.IsProfilePairBlockedResponse, error) {
	viewerProfileID, err := parseUUIDField("viewer_profile_id", req.GetViewerProfileId())
	if err != nil {
		return nil, err
	}
	otherProfileID, err := parseUUIDField("other_profile_id", req.GetOtherProfileId())
	if err != nil {
		return nil, err
	}
	if viewerProfileID == otherProfileID {
		return &socialv1.IsProfilePairBlockedResponse{Blocked: false}, nil
	}
	if s == nil || s.Blocks == nil || s.ProfileAccounts == nil {
		return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
	}
	viewerAccountID, err := s.ProfileAccounts.AccountIDByProfileID(ctx, viewerProfileID)
	if err != nil || viewerAccountID == uuid.Nil {
		return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
	}
	otherAccountID, err := s.ProfileAccounts.AccountIDByProfileID(ctx, otherProfileID)
	if err != nil || otherAccountID == uuid.Nil {
		return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
	}
	if viewerAccountID == otherAccountID {
		return &socialv1.IsProfilePairBlockedResponse{Blocked: false}, nil
	}
	blocked, err := s.Blocks.DirectedBlockExists(ctx, viewerAccountID, otherAccountID)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
	}
	return &socialv1.IsProfilePairBlockedResponse{Blocked: blocked}, nil
}

// IsProfilePairsBlocked evaluates a bounded set of directional profile pairs.
// Social keeps account ownership private and batches both User resolution and
// the ordered block-store lookup.
func (s *SocialGRPC) IsProfilePairsBlocked(ctx context.Context, req *socialv1.IsProfilePairsBlockedRequest) (*socialv1.IsProfilePairsBlockedResponse, error) {
	viewerProfileID, err := parseUUIDField("viewer_profile_id", req.GetViewerProfileId())
	if err != nil {
		return nil, err
	}
	if len(req.GetOtherProfileIds()) > maxProfilePairBlockBatch {
		return nil, status.Error(codes.InvalidArgument, "profile pair batch too large")
	}
	otherProfileIDs := make([]uuid.UUID, len(req.GetOtherProfileIds()))
	uniqueProfileIDs := make([]uuid.UUID, 0, len(otherProfileIDs))
	seenProfiles := make(map[uuid.UUID]struct{}, len(otherProfileIDs))
	for i, raw := range req.GetOtherProfileIds() {
		profileID, err := parseUUIDField("other_profile_id", raw)
		if err != nil {
			return nil, err
		}
		otherProfileIDs[i] = profileID
		if profileID == viewerProfileID {
			continue
		}
		if _, ok := seenProfiles[profileID]; !ok {
			seenProfiles[profileID] = struct{}{}
			uniqueProfileIDs = append(uniqueProfileIDs, profileID)
		}
	}
	if len(otherProfileIDs) == 0 {
		return &socialv1.IsProfilePairsBlockedResponse{}, nil
	}
	if s == nil || s.Blocks == nil || s.ProfileAccounts == nil {
		return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
	}
	viewerAccountID, err := s.ProfileAccounts.AccountIDByProfileID(ctx, viewerProfileID)
	if err != nil || viewerAccountID == uuid.Nil {
		return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
	}
	resolver, ok := s.ProfileAccounts.(ProfileAccountsBatchResolver)
	if !ok {
		return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
	}
	accountsByProfile, err := resolver.AccountIDsByProfileIDs(ctx, uniqueProfileIDs)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
	}
	accountsToCheck := make([]uuid.UUID, 0, len(uniqueProfileIDs))
	profileByAccount := make(map[uuid.UUID][]uuid.UUID, len(uniqueProfileIDs))
	for _, profileID := range uniqueProfileIDs {
		accountID, ok := accountsByProfile[profileID]
		if !ok || accountID == uuid.Nil {
			return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
		}
		if accountID == viewerAccountID {
			continue
		}
		if _, ok := profileByAccount[accountID]; !ok {
			accountsToCheck = append(accountsToCheck, accountID)
		}
		profileByAccount[accountID] = append(profileByAccount[accountID], profileID)
	}
	blockedAccounts := map[uuid.UUID]struct{}{}
	if len(accountsToCheck) > 0 {
		blockedAccounts, err = s.Blocks.DirectedBlocksExist(ctx, viewerAccountID, accountsToCheck)
		if err != nil {
			return nil, status.Error(codes.Unavailable, "message visibility policy unavailable")
		}
	}
	blockedProfiles := make(map[uuid.UUID]bool, len(uniqueProfileIDs))
	for accountID, profileIDs := range profileByAccount {
		_, blocked := blockedAccounts[accountID]
		for _, profileID := range profileIDs {
			blockedProfiles[profileID] = blocked
		}
	}
	results := make([]*socialv1.ProfilePairBlockResult, len(otherProfileIDs))
	for i, profileID := range otherProfileIDs {
		results[i] = &socialv1.ProfilePairBlockResult{OtherProfileId: profileID.String(), Blocked: blockedProfiles[profileID]}
	}
	return &socialv1.IsProfilePairsBlockedResponse{Results: results}, nil
}
