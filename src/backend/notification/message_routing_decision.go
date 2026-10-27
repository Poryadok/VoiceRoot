package main

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"sort"
	"sync"
	eventsv1 "voice.app/voice/events/v1"

	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/pushenrich"
	"voice/backend/pkg/notificationrouting"
)

type notificationRecipientAuthorizer interface {
	Authorize(context.Context, string, string, string) (bool, error)
}

func messageRoutingDecision(ctx context.Context, data []byte, members chatmembers.Lister, pusher *dispatch.MessagePusher, enrich pushenrich.Resolver, authority notificationRecipientAuthorizer) ([]notificationrouting.Target, error) {
	if len(data) == 0 || len(data) > notificationrouting.MaxBytes || authority == nil || pusher == nil {
		return nil, errors.New("recipient routing unavailable")
	}
	var event eventsv1.MessageStreamEvent
	if e := proto.Unmarshal(data, &event); e != nil {
		return nil, e
	}
	if event.GetEventId() == "" {
		return nil, errors.New("missing routing event identity")
	}
	var chat, sender, message string
	wanted := map[string]delivery.NotificationType{}
	var allMembers bool
	switch p := event.GetPayload().(type) {
	case *eventsv1.MessageStreamEvent_MessageSent:
		if p.MessageSent == nil {
			return nil, errors.New("missing message")
		}
		chat, sender, message = p.MessageSent.GetChatId(), p.MessageSent.GetSenderProfileId(), p.MessageSent.GetMessageId()
		allMembers = true
	case *eventsv1.MessageStreamEvent_MentionAdded:
		if p.MentionAdded == nil {
			return nil, errors.New("missing mention")
		}
		chat, sender, message = p.MentionAdded.GetChatId(), p.MentionAdded.GetSenderProfileId(), p.MentionAdded.GetMessageId()
		for _, id := range p.MentionAdded.GetMentionedProfileIds() {
			wanted[id] = delivery.TypeMention
		}
	case *eventsv1.MessageStreamEvent_ReactionAdded:
		if p.ReactionAdded == nil || p.ReactionAdded.GetEmoji() == "" {
			return nil, errors.New("missing reaction")
		}
		chat, sender, message = p.ReactionAdded.GetChatId(), p.ReactionAdded.GetProfileId(), p.ReactionAdded.GetMessageId()
		author := p.ReactionAdded.GetMessageAuthorProfileId()
		if author == "" {
			var e error
			author, e = parentMessageAuthor(ctx, enrich, message)
			if e != nil {
				return nil, e
			}
		}
		wanted[author] = delivery.TypeReaction
	default:
		return nil, nil
	}
	for _, id := range []string{chat, sender, message} {
		parsed, e := uuid.Parse(id)
		if e != nil || parsed == uuid.Nil || parsed.String() != id {
			return nil, errors.New("invalid routing identity")
		}
	}
	rows, e := listChatMembers(ctx, members, chat)
	if e != nil {
		return nil, e
	}
	if e = validateMessageMemberRows(rows, sender); e != nil {
		return nil, e
	}
	senderAllowed, e := authority.Authorize(ctx, sender, sender, chat)
	if e != nil {
		return nil, e
	}
	if !senderAllowed {
		return nil, nil
	}
	type candidate struct {
		profile string
		typ     delivery.NotificationType
	}
	candidates := make([]candidate, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		if seen[row.ProfileID] {
			return nil, errors.New("duplicate routing member")
		}
		seen[row.ProfileID] = true
		if row.ProfileID == sender || row.IsArchived || row.InboxBucket == "declined" {
			continue
		}
		typ, ok := wanted[row.ProfileID]
		if allMembers {
			typ = notificationTypeForInbox(row.InboxBucket)
			ok = true
		}
		if ok {
			candidates = append(candidates, candidate{row.ProfileID, typ})
		}
	}
	type result struct {
		target  notificationrouting.Target
		allowed bool
		err     error
	}
	results := make(chan result, len(candidates))
	jobs := make(chan candidate)
	var wg sync.WaitGroup
	for i := 0; i < 16 && i < len(candidates); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				allowed, e := authority.Authorize(ctx, item.profile, sender, chat)
				if e != nil || !allowed {
					results <- result{err: e}
					continue
				}
				decision, e := pusher.EnrichDecision(ctx, item.profile, uuid.MustParse(sender), chat, item.typ)
				results <- result{target: notificationrouting.Target{ProfileID: item.profile, Type: string(item.typ)}, allowed: decision.InApp, err: e}
			}
		}()
	}
	go func() {
		for _, item := range candidates {
			jobs <- item
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	var out []notificationrouting.Target
	var failed error
	for r := range results {
		if r.err != nil {
			failed = errors.Join(failed, r.err)
		}
		if r.allowed {
			out = append(out, r.target)
		}
	}
	if failed != nil {
		return nil, failed
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProfileID < out[j].ProfileID })
	return out, nil
}
