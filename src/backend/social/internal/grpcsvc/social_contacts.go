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

	"voice/backend/social/internal/authctx"
	"voice/backend/social/internal/store"

	socialv1 "voice.app/voice/social/v1"
)

const (
	contactsDefaultPage = 20
	contactsMaxPage     = 50
)

type contactsCursorPayload struct {
	UpdatedAtUnixNano int64  `json:"ts"`
	ID                string `json:"id"`
}

func encodeContactsCursor(updatedAt int64, id uuid.UUID) (string, error) {
	p := contactsCursorPayload{UpdatedAtUnixNano: updatedAt, ID: id.String()}
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return "c1." + base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeContactsCursor(s string) (*store.ContactsListCursor, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if !strings.HasPrefix(s, "c1.") {
		return nil, errors.New("invalid contacts cursor")
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(s, "c1."))
	if err != nil {
		return nil, errors.New("invalid contacts cursor")
	}
	var p contactsCursorPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, errors.New("invalid contacts cursor")
	}
	id, err := uuid.Parse(p.ID)
	if err != nil {
		return nil, errors.New("invalid contacts cursor")
	}
	return &store.ContactsListCursor{UpdatedAt: unixNanoToTime(p.UpdatedAtUnixNano), ID: id}, nil
}

func unixNanoToTime(n int64) time.Time {
	return time.Unix(0, n).UTC()
}

func (s *SocialGRPC) AddContact(ctx context.Context, req *socialv1.AddContactRequest) (*socialv1.AddContactResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	target, err := parseUUIDField("target_profile_id", req.GetTargetProfileId())
	if err != nil {
		return nil, err
	}
	if s.Contacts == nil {
		return nil, status.Error(codes.FailedPrecondition, "contacts not configured")
	}
	source := strings.TrimSpace(req.GetSource())
	if source == "" {
		source = "manual"
	}
	if err := s.Contacts.UpsertContact(ctx, caller, target, source, false); err != nil {
		if strings.Contains(err.Error(), "cannot add self") {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &socialv1.AddContactResponse{}, nil
}

func (s *SocialGRPC) RemoveContact(ctx context.Context, req *socialv1.RemoveContactRequest) (*socialv1.RemoveContactResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	target, err := parseUUIDField("target_profile_id", req.GetTargetProfileId())
	if err != nil {
		return nil, err
	}
	if s.Contacts == nil {
		return nil, status.Error(codes.FailedPrecondition, "contacts not configured")
	}
	if err := s.Contacts.RemoveContact(ctx, caller, target); err != nil {
		if errors.Is(err, store.ErrContactNotFound) {
			return nil, status.Error(codes.NotFound, "contact not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &socialv1.RemoveContactResponse{}, nil
}

func (s *SocialGRPC) ListContacts(ctx context.Context, req *socialv1.ListContactsRequest) (*socialv1.ListContactsResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	if s.Contacts == nil {
		return nil, status.Error(codes.FailedPrecondition, "contacts not configured")
	}
	pageSize := contactsDefaultPage
	cursorIn := ""
	if page := req.GetPage(); page != nil {
		if page.GetPageSize() > 0 {
			pageSize = int(page.GetPageSize())
		}
		cursorIn = page.GetCursor()
	}
	if pageSize > contactsMaxPage {
		pageSize = contactsMaxPage
	}
	after, err := decodeContactsCursor(cursorIn)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	rows, err := s.Contacts.ListContacts(ctx, caller, after, pageSize)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}
	out := make([]*socialv1.Contact, 0, len(rows))
	var next string
	for i, r := range rows {
		out = append(out, &socialv1.Contact{
			ProfileId:  r.ContactProfileID.String(),
			Source:     r.Source,
			IsFavorite: r.IsFavorite,
		})
		if hasMore && i == len(rows)-1 {
			next, err = encodeContactsCursor(r.UpdatedAt.UnixNano(), r.ID)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
	}
	return &socialv1.ListContactsResponse{
		ContactList: &socialv1.ContactList{Contacts: out, NextCursor: next},
	}, nil
}

func (s *SocialGRPC) SetFavorite(ctx context.Context, req *socialv1.SetFavoriteRequest) (*socialv1.SetFavoriteResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	friend, err := parseUUIDField("friend_profile_id", req.GetFriendProfileId())
	if err != nil {
		return nil, err
	}
	if s.Contacts == nil {
		return nil, status.Error(codes.FailedPrecondition, "contacts not configured")
	}
	if err := s.Contacts.SetFavorite(ctx, caller, friend, req.GetFavorite()); err != nil {
		if errors.Is(err, store.ErrContactNotFound) {
			return nil, status.Error(codes.NotFound, "contact not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &socialv1.SetFavoriteResponse{}, nil
}

func (s *SocialGRPC) ListFavorites(ctx context.Context, _ *socialv1.ListFavoritesRequest) (*socialv1.ListFavoritesResponse, error) {
	caller, ok := authctx.ProfileID(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing credentials")
	}
	if s.Contacts == nil {
		return nil, status.Error(codes.FailedPrecondition, "contacts not configured")
	}
	rows, err := s.Contacts.ListFavorites(ctx, caller)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	out := make([]*socialv1.FriendEdge, 0, len(rows))
	for _, r := range rows {
		out = append(out, &socialv1.FriendEdge{
			ProfileId: r.ContactProfileID.String(),
		})
	}
	return &socialv1.ListFavoritesResponse{
		FriendList: &socialv1.FriendList{Friends: out},
	}, nil
}
