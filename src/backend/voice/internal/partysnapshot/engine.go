package partysnapshot

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Snapshot brackets fresh Auth evidence with two independent native cuts. No
// native transaction or lock remains open while calling Auth.
func (e *Engine) Snapshot(ctx context.Context, subject Subject) (Snapshot, error) {
	if e == nil || e.Reader == nil || e.Floors == nil || subject.SearchOperationID == uuid.Nil || subject.AccountID == uuid.Nil || subject.ProfileID == uuid.Nil || subject.SessionEpoch <= 0 {
		return Snapshot{}, ErrUnavailable
	}
	now := e.Clock
	if now == nil {
		now = time.Now
	}
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return Snapshot{}, ErrUnavailable
		}
		first, err := e.Reader.Read(ctx, subject)
		if err != nil {
			return Snapshot{}, ErrUnavailable
		}
		members, err := eligible(first, subject)
		if err != nil {
			return Snapshot{}, err
		}
		accounts := map[uuid.UUID]int64{subject.AccountID: 0}
		for _, member := range members {
			accounts[member.AccountID] = 0
		}
		ids := make([]uuid.UUID, 0, len(accounts))
		for account := range accounts {
			ids = append(ids, account)
		}
		sortIDs(ids)
		for _, account := range ids {
			floor, err := e.Floors.CurrentFloor(ctx, account.String())
			if err != nil || floor <= 0 {
				return Snapshot{}, ErrUnavailable
			}
			accounts[account] = floor
		}
		if subject.SessionEpoch < accounts[subject.AccountID] {
			return Snapshot{}, ErrStaleSubject
		}
		changed, err := e.normalize(ctx, first, members, accounts, first.DatabaseTime, subject)
		if err != nil {
			return Snapshot{}, err
		}
		if changed {
			continue
		}
		started := now()
		second, err := e.Reader.Read(ctx, subject)
		if err != nil {
			return Snapshot{}, ErrUnavailable
		}
		if _, err = eligible(second, subject); err != nil {
			return Snapshot{}, err
		}
		if !sameCut(first, second) {
			continue
		}
		// Conservatively charge the whole read/validation duration after the DB
		// sample. Clock is monotonic in production; backward clocks fail closed.
		elapsed := now().Sub(started)
		if elapsed < 0 || second.DatabaseTime.Before(first.DatabaseTime) {
			return Snapshot{}, ErrUnavailable
		}
		changed, err = e.normalize(ctx, second, members, accounts, second.DatabaseTime, subject)
		if err != nil {
			return Snapshot{}, err
		}
		if changed {
			continue
		}
		elapsed = now().Sub(started)
		if elapsed < 0 {
			return Snapshot{}, ErrUnavailable
		}
		for _, member := range members {
			if member.State == Reconnecting && !second.DatabaseTime.Add(elapsed).Before(member.ReconnectDeadline) {
				// Local elapsed time cannot authorize a native expiry mutation.
				// Obtain a fresh DB clock and fresh Auth evidence on the next cut.
				changed = true
			}
		}
		if changed {
			continue
		}
		if ctx.Err() != nil {
			return Snapshot{}, ErrUnavailable
		}
		return buildSnapshot(subject, second)
	}
	return Snapshot{}, ErrUnavailable
}

func eligible(c Candidate, subject Subject) ([]Member, error) {
	if c.Coverage.CoverageID == uuid.Nil || c.Coverage.AuthorityIncarnation == uuid.Nil || c.Coverage.CoverageEpoch == 0 || c.Coverage.HeadRevision == 0 || c.Coverage.ReceiptHash == [32]byte{} || c.Fingerprint == [32]byte{} || c.DatabaseTime.IsZero() {
		return nil, ErrUnavailable
	}
	if c.Disposition == Absent {
		if c.SourceKind != "" || c.RoomID != uuid.Nil || c.Incarnation != uuid.Nil || c.RosterVersion != 0 || len(c.Members) != 0 || c.Purpose != "" || c.RoomState != "" {
			return nil, ErrUnavailable
		}
		return nil, nil
	}
	if c.Disposition != Present {
		return nil, ErrUnavailable
	}
	if c.Purpose == "GAME" || c.Purpose == "MATCH_SQUAD" {
		return nil, ErrIneligible
	}
	if (c.SourceKind != "ORDINARY" && c.SourceKind != "SPACE") || c.Purpose != "ORDINARY" || c.RoomState != "ACTIVE" || c.RoomID == uuid.Nil || c.Incarnation == uuid.Nil || c.RosterVersion == 0 {
		return nil, ErrUnavailable
	}
	members := make([]Member, 0, len(c.Members))
	seen := make(map[uuid.UUID]bool, len(c.Members))
	seenAccounts := make(map[uuid.UUID]bool, len(c.Members))
	found := false
	for _, member := range c.Members {
		switch member.State {
		case Joining, Leaving, Left:
			continue
		case Joined, Reconnecting:
		default:
			return nil, ErrUnavailable
		}
		if member.AccountID == uuid.Nil || member.ProfileID == uuid.Nil || member.OperationID == uuid.Nil || member.Generation == "" || !utf8.ValidString(member.Generation) || uint64(len(member.Generation)) > uint64(^uint32(0)) || member.SessionEpoch <= 0 || seen[member.ProfileID] || seenAccounts[member.AccountID] {
			return nil, ErrUnavailable
		}
		seen[member.ProfileID] = true
		seenAccounts[member.AccountID] = true
		if member.State == Reconnecting {
			interval := member.ReconnectDeadline.Sub(member.ReconnectStartedAt)
			if member.ReconnectStartedAt.IsZero() || member.ReconnectDeadline.IsZero() || interval <= 0 || interval > 30*time.Second {
				return nil, ErrUnavailable
			}
		}
		if member.ProfileID == subject.ProfileID {
			if member.AccountID != subject.AccountID {
				return nil, ErrUnavailable
			}
			found = true
		}
		members = append(members, member)
	}
	if !found {
		return nil, ErrIneligible
	}
	return members, nil
}

func (e *Engine) normalize(ctx context.Context, c Candidate, members []Member, floors map[uuid.UUID]int64, at time.Time, subject Subject) (bool, error) {
	changed := false
	for _, member := range members {
		reason := ""
		if member.SessionEpoch < floors[member.AccountID] {
			reason = "SESSION_REVOKED"
		} else if member.State == Reconnecting && !at.Before(member.ReconnectDeadline) {
			reason = "RECONNECT_EXPIRED"
		}
		if reason == "" {
			continue
		}
		if e.Normalizer == nil {
			return false, ErrUnavailable
		}
		err := e.Normalizer.ApplyEligibilityLoss(ctx, LossTarget{SourceKind: c.SourceKind, RoomID: c.RoomID, Incarnation: c.Incarnation, Member: member, Reason: reason})
		if err != nil && !errors.Is(err, ErrSourceChanged) {
			return false, ErrUnavailable
		}
		// A stale initiator cannot acquire a SOLO result after its locator is
		// removed, even when a concurrent native mutation won the exact CAS.
		if member.ProfileID == subject.ProfileID {
			return true, ErrIneligible
		}
		changed = true
	}
	return changed, nil
}

func sameCut(a, b Candidate) bool {
	a.DatabaseTime, b.DatabaseTime = time.Time{}, time.Time{}
	a.Members, b.Members = append([]Member(nil), a.Members...), append([]Member(nil), b.Members...)
	less := func(m []Member) func(int, int) bool {
		return func(i, j int) bool { return bytes.Compare(m[i].ProfileID[:], m[j].ProfileID[:]) < 0 }
	}
	sort.Slice(a.Members, less(a.Members))
	sort.Slice(b.Members, less(b.Members))
	return reflect.DeepEqual(a, b)
}

func sortIDs(ids []uuid.UUID) {
	sort.Slice(ids, func(i, j int) bool { return bytes.Compare(ids[i][:], ids[j][:]) < 0 })
}
