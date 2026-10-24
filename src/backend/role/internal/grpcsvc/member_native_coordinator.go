package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"sort"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/authctx"
	"voice/backend/role/internal/memberauthority"
	"voice/backend/role/internal/store"
)

var errNativeAuthorityChanged = errors.New("native Role authority changed")

type nativeMemberChatScopeKey struct{}
type nativeMemberDecisionReadKey struct{}

// This is only the server's copy of the already bound request selector. It is
// not an observation or authorization and never comes from request metadata.
type nativeMemberChatScope struct {
	profile, chat string
	voicePresent  bool
}

// withNativeOrdinary keeps every remote owner call outside local locks. The
// first Role cut captures R, the second owner observation must match its exact
// vector, and the final local cut CASes R before authorization and effects.
func withNativeOrdinary(ctx context.Context, s *RoleGRPC, space uuid.UUID, targets []string, call func(context.Context, *store.RoleStore) error) error {
	native, err := s.Store.NativeMembershipRequired(ctx)
	if err != nil {
		return err
	}
	if !native {
		return s.Store.WithinSpaces(ctx, []uuid.UUID{space}, func(scoped *store.RoleStore) error { return call(ctx, scoped) })
	}
	if _, ok := principal.FromContext(ctx); !ok {
		return status.Error(codes.Unauthenticated, "verified native Role principal required")
	}
	profiles := map[string]bool{}
	if actor, ok := authctx.ProfileID(ctx); ok {
		profiles[actor.String()] = true
	}
	for _, raw := range targets {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || id.String() != raw {
			return status.Error(codes.InvalidArgument, "canonical target profile required")
		}
		profiles[raw] = true
	}
	chatScope, scopedChat := ctx.Value(nativeMemberChatScopeKey{}).(nativeMemberChatScope)
	if scopedChat {
		chat, err := uuid.Parse(chatScope.chat)
		if err != nil || chat == uuid.Nil || chat.String() != chatScope.chat || !profiles[chatScope.profile] || chatScope.voicePresent {
			return status.Error(codes.InvalidArgument, "one canonical member chat scope required")
		}
	}
	// Bootstrap and trusted definition inventory have no member subject. These
	// operations do not derive member permissions or manufacture membership.
	if len(profiles) == 0 {
		return s.Store.WithinSpaces(ctx, []uuid.UUID{space}, func(scoped *store.RoleStore) error { return call(ctx, scoped) })
	}
	if s.MemberAuthority == nil {
		return status.Error(codes.Unavailable, "native membership authority unavailable")
	}
	ids := make([]string, 0, len(profiles))
	for id := range profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	observe := func() ([]memberauthority.Observation, error) {
		out := make([]memberauthority.Observation, 0, len(ids)+1)
		for _, id := range ids {
			o, err := s.MemberAuthority.ObserveMember(bounded, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.String(), ProfileId: id})
			if err != nil {
				return nil, err
			}
			out = append(out, o)
		}
		if scopedChat {
			chat := chatScope.chat
			o, err := s.MemberAuthority.ObserveMember(bounded, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.String(), ProfileId: chatScope.profile, ChatId: &chat})
			if err != nil {
				return nil, err
			}
			if !o.BoundToChat(chat, time.Now()) {
				return nil, store.ErrScopeUnavailable
			}
			out = append(out, o)
		}
		if _, err := memberauthority.WithObservations(bounded, out...); err != nil {
			return nil, err
		}
		return out, nil
	}
	for attempt := 0; attempt < 2; attempt++ {
		first, err := observe()
		if err != nil {
			return memberAdmissionError(bounded, err)
		}
		var revision uint64
		err = s.Store.WithinSpaces(bounded, []uuid.UUID{space}, func(scoped *store.RoleStore) error {
			var e error
			revision, e = scoped.QualifiedRevision(bounded, space)
			return e
		})
		if err != nil {
			return err
		}
		second, err := observe()
		if err != nil {
			return memberAdmissionError(bounded, err)
		}
		same := true
		for i := range first {
			same = same && first[i].SameSnapshot(second[i], time.Now())
		}
		if !same {
			continue
		}
		observed, err := memberauthority.WithObservations(bounded, second...)
		if err != nil {
			return memberAdmissionError(bounded, err)
		}
		err = s.Store.WithinSpaces(observed, []uuid.UUID{space}, func(scoped *store.RoleStore) error {
			current, err := scoped.QualifiedRevision(observed, space)
			if err != nil {
				return err
			}
			if current != revision {
				return errNativeAuthorityChanged
			}
			for _, o := range second {
				if !o.Available(time.Now()) {
					return store.ErrScopeUnavailable
				}
			}
			if _, ok := authctx.ProfileID(observed); ok {
				for _, id := range ids {
					present, err := scoped.QualifiedMemberPresent(observed, space, uuid.MustParse(id))
					if err != nil {
						return err
					}
					if !present {
						return status.Error(codes.PermissionDenied, "current Space membership required for actor and target")
					}
				}
			}
			if err = call(observed, scoped); err != nil {
				return err
			}
			for _, o := range second {
				if !o.Available(time.Now()) {
					return store.ErrScopeUnavailable
				}
			}
			return nil
		})
		if errors.Is(err, errNativeAuthorityChanged) {
			continue
		}
		// A scoped chat request is a read. Its Store is nested in this owning
		// transaction, so Store-level checks precede this outer COMMIT. Recheck
		// the same immutable vector after it completes; never use this rule to
		// reinterpret a successfully committed mutation as a failed mutation.
		decisionRead, _ := ctx.Value(nativeMemberDecisionReadKey{}).(bool)
		if err == nil && (scopedChat || decisionRead) {
			for _, o := range second {
				if !o.Available(time.Now()) {
					return store.ErrScopeUnavailable
				}
			}
		}
		return err
	}
	return status.Error(codes.Unavailable, "native Role authority changed during bounded decision")
}
