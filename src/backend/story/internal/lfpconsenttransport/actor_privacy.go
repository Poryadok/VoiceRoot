package lfpconsenttransport

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"voice/backend/pkg/lfpconsent"
	"voice/backend/pkg/principal"
	"voice/backend/pkg/privacy"
	"voice/backend/story/internal/lfpconsentstore"
)

type ActorPrivacyFloor interface {
	ShowStoriesAudience(context.Context, uuid.UUID) (privacy.Audience, error)
}

// ActorPrivacy applies the same Story floor + audience matcher to an explicit
// original human viewer. It never derives guest status from service metadata,
// a current owner lookup or compatibility account-type defaults.
type ActorPrivacy struct {
	Floor   ActorPrivacyFloor
	Matcher privacy.Matcher
}

func (p *ActorPrivacy) CheckActorResponder(ctx context.Context, bound lfpconsentstore.OfferContext, actor lfpconsent.Actor) error {
	if p == nil || p.Floor == nil {
		return lfpconsentstore.ErrUnavailable
	}
	if _, _, e := lfpconsent.EncodeActor(actor); e != nil || actor.ActionKind != lfpconsent.Respond || bound.Offer().StoryID == uuid.Nil || bound.Offer().AuthorProfileID == uuid.Nil {
		return lfpconsentstore.ErrUnavailable
	}
	return p.checkViewer(ctx, bound, actor.ProfileID, actor.AccountType == lfpconsent.Guest)
}

// CheckHumanOfferViewer is not a persisted decision and has no invented actor-codec kind.
// Its exact verifier identity still supplies an explicit human viewer type.
func (p *ActorPrivacy) CheckHumanOfferViewer(ctx context.Context, bound lfpconsentstore.OfferContext, viewer principal.UserIdentity) error {
	if viewer.AccountID == uuid.Nil || viewer.ProfileID == uuid.Nil || viewer.SessionEpoch <= 0 || (viewer.AccountType != "regular" && viewer.AccountType != "guest") {
		return lfpconsentstore.ErrUnavailable
	}
	return p.checkViewer(ctx, bound, viewer.ProfileID, viewer.AccountType == "guest")
}

func (p *ActorPrivacy) checkViewer(ctx context.Context, bound lfpconsentstore.OfferContext, viewer uuid.UUID, guest bool) error {
	if p == nil || p.Floor == nil || bound.Offer().StoryID == uuid.Nil || bound.Offer().AuthorProfileID == uuid.Nil {
		return lfpconsentstore.ErrUnavailable
	}
	audience, e := strictActorAudience(bound.Visibility(), bound.AudienceJSON())
	if e != nil {
		return e
	}
	floor, e := p.Floor.ShowStoriesAudience(ctx, bound.Offer().AuthorProfileID)
	if e != nil || privacy.ValidateAudience("show_stories", floor) != nil {
		return lfpconsentstore.ErrUnavailable
	}
	for _, a := range []privacy.Audience{floor, audience} {
		allowed, e := p.Matcher.Allowed(ctx, bound.Offer().AuthorProfileID, viewer, a, guest)
		if e != nil {
			return lfpconsentstore.ErrUnavailable
		}
		if allowed {
			continue
		}
		// A missing social/space authority cannot establish a definite denial.
		if !guest && ((a.Friends || a.FriendsOfFriends) && p.Matcher.Social == nil || a.SpaceMembers && p.Matcher.Space == nil) {
			return lfpconsentstore.ErrUnavailable
		}
		return ErrResponderIneligible
	}
	return nil
}

func strictActorAudience(visibility, raw string) (privacy.Audience, error) {
	switch visibility {
	case "everyone", "friends", "close_friends", "custom":
	default:
		return privacy.Audience{}, lfpconsentstore.ErrUnavailable
	}
	if raw != "" {
		// PostgreSQL supplied one complete JSONB value. Reject malformed/null or
		// unrecognized shape instead of defaulting to a broader legacy audience.
		if !strings.HasPrefix(strings.TrimSpace(raw), "{") {
			return privacy.Audience{}, lfpconsentstore.ErrUnavailable
		}
		var audience privacy.Audience
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&audience) != nil || privacy.ValidateAudience("story", audience) != nil {
			return privacy.Audience{}, lfpconsentstore.ErrUnavailable
		}
		return audience, nil
	}
	switch visibility {
	case "everyone":
		return privacy.EveryoneWithGuests(), nil
	case "friends":
		return privacy.FriendsOnly(), nil
	case "close_friends":
		return privacy.FriendsAndFoF(), nil
	default:
		return privacy.Nobody(), nil
	}
}
