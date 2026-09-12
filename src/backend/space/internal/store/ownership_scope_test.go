package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type r20ScopeFixture struct {
	binding                                      OwnershipBinding
	owner, currentOwner, member, joiner, account uuid.UUID
	invite                                       *InviteRow
	category                                     *CategoryRow
	voice                                        *VoiceRoomRow
	voiceNode, textNode                          *TreeNodeRow
	auditID, bannedAccount, timedProfile         uuid.UUID
}

func ownershipScopeStoreFixture(t *testing.T) *SpaceStore {
	t.Helper()
	st := ownershipJournalCommitStoreFixture(t)
	applyR22SpaceEpochMigration(t, context.Background(), st)
	return st
}

func newR20ScopeFixture(t *testing.T, st *SpaceStore, state string) r20ScopeFixture {
	t.Helper()
	f := r20ScopeFixture{binding: seedOwnershipJournalBinding(t, st)}
	f.owner, f.currentOwner = f.binding.ActorProfileID, f.binding.ActorProfileID
	f.member, f.joiner, f.account = uuid.New(), uuid.New(), uuid.New()
	f.bannedAccount, f.timedProfile = uuid.New(), uuid.New()
	for _, profile := range []uuid.UUID{f.member, f.timedProfile} {
		_, err := st.Pool.Exec(context.Background(), `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, f.binding.SpaceID, profile)
		require.NoError(t, err)
	}
	invite, err := st.CreateInvite(context.Background(), CreateInviteInput{SpaceID: f.binding.SpaceID, CreatorProfileID: f.owner})
	require.NoError(t, err)
	f.invite = invite
	f.category, err = st.CreateCategory(context.Background(), f.binding.SpaceID, "r20 category", 1)
	require.NoError(t, err)
	f.voice, f.voiceNode, err = st.CreateVoiceRoom(context.Background(), f.binding.SpaceID, "r20 room", &f.category.ID)
	require.NoError(t, err)
	chatID := uuid.New()
	f.textNode, err = st.UpsertTreeNode(context.Background(), UpsertTreeNodeInput{SpaceID: f.binding.SpaceID, CategoryID: &f.category.ID, Kind: TreeKindTextChat, ChatID: &chatID})
	require.NoError(t, err)
	f.auditID = uuid.New()
	_, err = st.Pool.Exec(context.Background(), `INSERT INTO audit_log(id,space_id,actor_profile_id,action,target_type,target_id,details) VALUES($1,$2,$3,'fixture','profile',$4,'{}')`, f.auditID, f.binding.SpaceID, f.owner, f.member)
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), `INSERT INTO space_bans(space_id,account_id,banned_by_profile_id,reason) VALUES($1,$2,$3,'fixture')`, f.binding.SpaceID, f.bannedAccount, f.owner)
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), `INSERT INTO space_member_timeouts(space_id,profile_id,timed_out_until,timed_out_by_profile_id,reason) VALUES($1,$2,now()+interval '1 hour',$3,'fixture')`, f.binding.SpaceID, f.timedProfile, f.owner)
	require.NoError(t, err)
	require.NoError(t, st.UpsertSpaceSubscription(context.Background(), f.binding.SpaceID, f.account, "active"))
	existingBot := uuid.New()
	require.NoError(t, st.AddBotMember(context.Background(), f.binding.SpaceID, existingBot))
	f.joiner = existingBot

	if state == "" {
		return f
	}
	_, err = st.ReserveOwnership(context.Background(), f.binding)
	require.NoError(t, err)
	if state == "reserved" {
		return f
	}
	_, err = st.ConfirmOwnershipProof(context.Background(), f.binding, ownershipAuthReceiptFixture(f.binding))
	require.NoError(t, err)
	if state == "proof_confirmed" {
		return f
	}
	_, err = st.MarkOwnershipPrepared(context.Background(), f.binding, ownershipRolePreparedReceiptFixture(f.binding))
	require.NoError(t, err)
	if state == "prepared" {
		return f
	}
	if state == "commit_decided" {
		_, err = st.DecideOwnershipCommit(context.Background(), f.binding)
		require.NoError(t, err)
		f.currentOwner = f.binding.NewOwnerProfileID
		return f
	}
	_, err = st.DecideOwnershipAbort(context.Background(), f.binding)
	require.NoError(t, err)
	require.Equal(t, "abort_decided", state)
	return f
}

func r20ScopeSnapshot(t *testing.T, st *SpaceStore, spaceID uuid.UUID) string {
	t.Helper()
	var snapshot string
	require.NoError(t, st.Pool.QueryRow(context.Background(), `SELECT jsonb_build_object(
		'space',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM spaces x WHERE id=$1),
		'members',(SELECT jsonb_agg(to_jsonb(x) ORDER BY profile_id) FROM space_members x WHERE space_id=$1),
		'invites',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM invites x WHERE space_id=$1),
		'bans',(SELECT jsonb_agg(to_jsonb(x) ORDER BY account_id) FROM space_bans x WHERE space_id=$1),
		'timeouts',(SELECT jsonb_agg(to_jsonb(x) ORDER BY profile_id) FROM space_member_timeouts x WHERE space_id=$1),
		'subscriptions',(SELECT jsonb_agg(to_jsonb(x) ORDER BY purchaser_account_id) FROM space_subscriptions x WHERE space_id=$1),
		'categories',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM categories x WHERE space_id=$1),
		'rooms',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM voice_rooms x WHERE space_id=$1),
		'nodes',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM space_tree_nodes x WHERE space_id=$1),
		'audit',(SELECT jsonb_agg(to_jsonb(x) ORDER BY id) FROM audit_log x WHERE space_id=$1),
		'journal',(SELECT jsonb_agg(to_jsonb(x) ORDER BY operation_id) FROM ownership_journal x WHERE space_id=$1),
		'ownership_outbox',(SELECT jsonb_agg(to_jsonb(x) ORDER BY event_id) FROM ownership_outbox x WHERE space_id=$1),
		'access_epoch',(SELECT jsonb_agg(to_jsonb(x) ORDER BY space_id) FROM space_voice_access_epochs x WHERE space_id=$1),
		'access_outbox',(SELECT jsonb_agg(to_jsonb(x) ORDER BY access_epoch) FROM space_voice_access_outbox x WHERE space_id=$1)
	)::text`, spaceID).Scan(&snapshot))
	return snapshot
}

type r20ScopeCase struct {
	name string
	call func(*SpaceStore, r20ScopeFixture) error
}

func r20OrdinaryScopeCases() []r20ScopeCase {
	return []r20ScopeCase{
		{"GetSpace", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.GetSpace(context.Background(), f.binding.SpaceID)
			return err
		}},
		{"IsSpaceMember", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.IsSpaceMember(context.Background(), f.binding.SpaceID, f.member)
			return err
		}},
		{"UpdateSpace", func(s *SpaceStore, f r20ScopeFixture) error {
			name := "changed"
			_, err := s.UpdateSpace(context.Background(), f.binding.SpaceID, UpdateSpaceInput{Name: &name})
			return err
		}},
		{"DeleteSpace", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.DeleteSpace(context.Background(), f.binding.SpaceID)
		}},
		{"TransferOwnership", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.TransferOwnership(context.Background(), f.binding.SpaceID, f.currentOwner, f.member)
		}},
		{"RecordOwnershipTransferred", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.RecordOwnershipTransferred(context.Background(), uuid.New(), f.binding.SpaceID, f.owner, f.binding.NewOwnerProfileID)
		}},
		{"DeleteAuditLogEntry", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.DeleteAuditLogEntry(context.Background(), f.auditID)
		}},
		{"ListMySpacesPage", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.ListMySpacesPage(context.Background(), f.member, "", 10)
			return err
		}},
		{"ListSpaceMembersPage", func(s *SpaceStore, f r20ScopeFixture) error {
			_, _, err := s.ListSpaceMembersPage(context.Background(), f.binding.SpaceID, 10, "")
			return err
		}},
		{"RemoveMember", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.RemoveMember(context.Background(), f.binding.SpaceID, f.member)
		}},
		{"RecordMemberKicked", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.RecordMemberKicked(context.Background(), f.binding.SpaceID, f.member, f.currentOwner)
		}},
		{"GetMembership", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.GetMembership(context.Background(), f.binding.SpaceID, f.member)
			return err
		}},
		{"AreCoMembers", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.AreCoMembers(context.Background(), f.owner, f.member, []uuid.UUID{f.binding.SpaceID})
			return err
		}},
		{"CreateInvite", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.CreateInvite(context.Background(), CreateInviteInput{SpaceID: f.binding.SpaceID, CreatorProfileID: f.currentOwner})
			return err
		}},
		{"ListInvites", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.ListInvites(context.Background(), f.binding.SpaceID)
			return err
		}},
		{"GetInviteByCode", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.GetInviteByCode(context.Background(), f.invite.Code)
			return err
		}},
		{"GetInviteByID", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.GetInviteByID(context.Background(), f.invite.ID)
			return err
		}},
		{"RevokeInvite", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.RevokeInvite(context.Background(), f.invite.ID, f.currentOwner)
		}},
		{"AllowGuestsForInvite", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.AllowGuestsForInvite(context.Background(), f.invite.Code)
			return err
		}},
		{"JoinByInvite", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.JoinByInvite(context.Background(), f.invite.Code, uuid.New(), uuid.New())
			return err
		}},
		{"JoinSpace", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.JoinSpace(context.Background(), f.binding.SpaceID, uuid.New(), uuid.New())
			return err
		}},
		{"IsAccountBanned", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.IsAccountBanned(context.Background(), f.binding.SpaceID, f.bannedAccount)
			return err
		}},
		{"ListBans", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.ListBans(context.Background(), f.binding.SpaceID)
			return err
		}},
		{"BanMember", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.BanMember(context.Background(), f.binding.SpaceID, uuid.New(), f.currentOwner, nil, &f.member)
		}},
		{"UnbanMember", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.UnbanMember(context.Background(), f.binding.SpaceID, f.bannedAccount, f.currentOwner)
		}},
		{"SetMemberTimeout", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.SetMemberTimeout(context.Background(), f.binding.SpaceID, f.member, f.currentOwner, 60, nil)
		}},
		{"RemoveMemberTimeout", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.RemoveMemberTimeout(context.Background(), f.binding.SpaceID, f.timedProfile, f.currentOwner)
		}},
		{"IsProfileTimedOut", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.IsProfileTimedOut(context.Background(), f.binding.SpaceID, f.timedProfile)
			return err
		}},
		{"AddBotMember", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.AddBotMember(context.Background(), f.binding.SpaceID, uuid.New())
		}},
		{"RemoveBotMember", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.RemoveBotMember(context.Background(), f.binding.SpaceID, f.joiner)
		}},
		{"HasActiveSpacePro", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.HasActiveSpacePro(context.Background(), f.binding.SpaceID)
			return err
		}},
		{"MemberCap", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.MemberCap(context.Background(), f.binding.SpaceID)
			return err
		}},
		{"UpsertSpaceSubscription", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.UpsertSpaceSubscription(context.Background(), f.binding.SpaceID, f.account, "grace_period")
		}},
		{"SyncSpaceProSubscription", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.SyncSpaceProSubscription(context.Background(), f.binding.SpaceID, f.account, "grace_period")
		}},
		{"FinalizeSpacePro", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.FinalizeSpacePro(context.Background(), f.binding.SpaceID)
		}},
		{"ListAuditLogSignedPage", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.ListAuditLogSignedPage(context.Background(), f.binding.SpaceID, "", 10, "r20-test", bytes.Repeat([]byte{0xa7}, 32))
			return err
		}},
		{"CreateCategory", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.CreateCategory(context.Background(), f.binding.SpaceID, "new", 2)
			return err
		}},
		{"ListCategories", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.ListCategories(context.Background(), f.binding.SpaceID)
			return err
		}},
		{"UpdateCategory", func(s *SpaceStore, f r20ScopeFixture) error {
			name := "changed"
			_, err := s.UpdateCategory(context.Background(), f.category.ID, &name, nil)
			return err
		}},
		{"DeleteCategory", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.DeleteCategory(context.Background(), f.category.ID)
		}},
		{"CreateVoiceRoom", func(s *SpaceStore, f r20ScopeFixture) error {
			_, _, err := s.CreateVoiceRoom(context.Background(), f.binding.SpaceID, "new room", &f.category.ID)
			return err
		}},
		{"ListVoiceRooms", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.ListVoiceRooms(context.Background(), f.binding.SpaceID)
			return err
		}},
		{"UpdateVoiceRoom", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.UpdateVoiceRoom(context.Background(), f.voice.ID, "changed")
			return err
		}},
		{"DeleteVoiceRoom", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.DeleteVoiceRoom(context.Background(), f.voice.ID)
		}},
		{"UpsertTreeNode", func(s *SpaceStore, f r20ScopeFixture) error {
			id := uuid.New()
			_, err := s.UpsertTreeNode(context.Background(), UpsertTreeNodeInput{SpaceID: f.binding.SpaceID, Kind: TreeKindTextChat, ChatID: &id})
			return err
		}},
		{"RemoveTreeNode", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.RemoveTreeNode(context.Background(), f.binding.SpaceID, f.textNode.ID)
		}},
		{"ListTreeNodes", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.ListTreeNodes(context.Background(), f.binding.SpaceID)
			return err
		}},
		{"ListSpaceTree", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.ListSpaceTree(context.Background(), f.binding.SpaceID)
			return err
		}},
		{"PinTreeNode", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.PinTreeNode(context.Background(), f.binding.SpaceID, f.textNode.ID)
			return err
		}},
		{"UnpinTreeNode", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.UnpinTreeNode(context.Background(), f.binding.SpaceID, f.voiceNode.ID)
			return err
		}},
		{"ReorderSpaceTree", func(s *SpaceStore, f r20ScopeFixture) error {
			return s.ReorderSpaceTree(context.Background(), f.binding.SpaceID, []uuid.UUID{f.textNode.ID, f.voiceNode.ID})
		}},
		{"ResolveVoiceRoomAccess", func(s *SpaceStore, f r20ScopeFixture) error {
			_, err := s.ResolveVoiceRoomAccess(context.Background(), f.binding.SpaceID, f.voice.ID, f.member)
			return err
		}},
	}
}

func TestOwnershipScope_EveryCanonicalOrdinaryMethodFailsClosedForEveryNonterminalState(t *testing.T) {
	st := ownershipScopeStoreFixture(t)
	for _, state := range []string{"reserved", "proof_confirmed", "prepared", "commit_decided", "abort_decided"} {
		for _, tc := range r20OrdinaryScopeCases() {
			t.Run(state+"/"+tc.name, func(t *testing.T) {
				f := newR20ScopeFixture(t, st, state)
				before := r20ScopeSnapshot(t, st, f.binding.SpaceID)
				err := tc.call(st, f)
				assert.Error(t, err, "pending ownership must fail closed")
				assert.Equal(t, before, r20ScopeSnapshot(t, st, f.binding.SpaceID), "denied ordinary action changed rows, epoch, outbox or audit")
			})
		}
	}
}

func TestOwnershipScope_OrdinaryFixtureCallsAreValidWithoutPendingJournal(t *testing.T) {
	st := ownershipScopeStoreFixture(t)
	for _, tc := range r20OrdinaryScopeCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := newR20ScopeFixture(t, st, "")
			require.NoError(t, tc.call(st, f), "fixture must exercise a normally valid path")
		})
	}
}

type r20IndirectMissingCase struct {
	name        string
	relation    string
	wantMissing error
	call        func(*SpaceStore, r20ScopeFixture) (bool, error)
}

func r20IndirectMissingCases() []r20IndirectMissingCase {
	return []r20IndirectMissingCase{
		{"GetInviteByCode", "invites", nil, func(s *SpaceStore, _ r20ScopeFixture) (bool, error) {
			row, err := s.GetInviteByCode(context.Background(), "missing")
			return row != nil, err
		}},
		{"GetInviteByID", "invites", nil, func(s *SpaceStore, _ r20ScopeFixture) (bool, error) {
			row, err := s.GetInviteByID(context.Background(), uuid.New())
			return row != nil, err
		}},
		{"RevokeInvite", "invites", ErrInviteNotFound, func(s *SpaceStore, f r20ScopeFixture) (bool, error) {
			return false, s.RevokeInvite(context.Background(), uuid.New(), f.currentOwner)
		}},
		{"JoinByInvite", "invites", ErrInviteNotFound, func(s *SpaceStore, f r20ScopeFixture) (bool, error) {
			row, err := s.JoinByInvite(context.Background(), "missing", f.joiner, f.account)
			return row != nil, err
		}},
		{"AllowGuestsForInvite", "invites", ErrInviteNotFound, func(s *SpaceStore, _ r20ScopeFixture) (bool, error) {
			return s.AllowGuestsForInvite(context.Background(), "missing")
		}},
		{"UpdateCategory", "categories", ErrCategoryNotFound, func(s *SpaceStore, _ r20ScopeFixture) (bool, error) {
			name := "missing"
			row, err := s.UpdateCategory(context.Background(), uuid.New(), &name, nil)
			return row != nil, err
		}},
		{"DeleteCategory", "categories", ErrCategoryNotFound, func(s *SpaceStore, _ r20ScopeFixture) (bool, error) {
			return false, s.DeleteCategory(context.Background(), uuid.New())
		}},
		{"UpdateVoiceRoom", "voice_rooms", ErrVoiceRoomNotFound, func(s *SpaceStore, _ r20ScopeFixture) (bool, error) {
			row, err := s.UpdateVoiceRoom(context.Background(), uuid.New(), "missing")
			return row != nil, err
		}},
		{"DeleteVoiceRoom", "voice_rooms", ErrVoiceRoomNotFound, func(s *SpaceStore, _ r20ScopeFixture) (bool, error) {
			return false, s.DeleteVoiceRoom(context.Background(), uuid.New())
		}},
		{"DeleteAuditLogEntry", "audit_log", nil, func(s *SpaceStore, _ r20ScopeFixture) (bool, error) {
			return false, s.DeleteAuditLogEntry(context.Background(), uuid.New())
		}},
	}
}

func TestOwnershipScope_IndirectMethodsPreserveProvenMissingSemantics(t *testing.T) {
	st := ownershipScopeStoreFixture(t)
	f := newR20ScopeFixture(t, st, "")
	for _, tc := range r20IndirectMissingCases() {
		t.Run(tc.name, func(t *testing.T) {
			nonzero, err := tc.call(st, f)
			require.False(t, nonzero)
			if tc.wantMissing == nil {
				require.NoError(t, err, "legacy missing-row no-op must remain successful")
				return
			}
			require.ErrorIs(t, err, tc.wantMissing, "legacy missing-row classification must survive indirect scope resolution")
		})
	}
}

func TestOwnershipScope_ResolverFailureReturnsErrorWithoutCallingAction(t *testing.T) {
	st := ownershipScopeStoreFixture(t)
	want := errors.New("injected resolver failure")
	actionCalled := false
	value, err := withResolvedOwnershipScopeValue(st, context.Background(), func(spaceStoreDB) (uuid.UUID, error) {
		return uuid.Nil, want
	}, func(*SpaceStore) (bool, error) {
		actionCalled = true
		return true, nil
	})
	require.ErrorIs(t, err, ErrOwnershipScopeUnavailable)
	require.False(t, value)
	require.False(t, actionCalled)
}

func TestOwnershipScope_IndirectResolverSchemaFailuresFailClosed(t *testing.T) {
	for _, tc := range r20IndirectMissingCases() {
		t.Run(tc.name, func(t *testing.T) {
			st := ownershipScopeStoreFixture(t)
			f := newR20ScopeFixture(t, st, "")
			unavailable := tc.relation + "_r20_unavailable"
			_, err := st.Pool.Exec(context.Background(), `ALTER TABLE `+pgx.Identifier{tc.relation}.Sanitize()+` RENAME TO `+pgx.Identifier{unavailable}.Sanitize())
			require.NoError(t, err)
			t.Cleanup(func() {
				_, restoreErr := st.Pool.Exec(context.Background(), `ALTER TABLE `+pgx.Identifier{unavailable}.Sanitize()+` RENAME TO `+pgx.Identifier{tc.relation}.Sanitize())
				require.NoError(t, restoreErr)
			})

			nonzero, callErr := tc.call(st, f)
			require.Error(t, callErr, "resolver schema failure must fail closed")
			require.ErrorIs(t, callErr, ErrOwnershipScopeUnavailable)
			require.False(t, nonzero)
			require.NotErrorIs(t, callErr, pgx.ErrNoRows)
			for _, missing := range []error{ErrInviteNotFound, ErrCategoryNotFound, ErrVoiceRoomNotFound} {
				require.NotErrorIs(t, callErr, missing, "schema failure must not be sanitized as proven missing")
			}
		})
	}
}

func TestOwnershipScope_ProvenMissingRevokeInviteIsStableAcrossConcurrentCreationAndReservation(t *testing.T) {
	st := ownershipScopeStoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	owner, newOwner := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "missing snapshot", "", "private")
	require.NoError(t, err)
	inviteID := uuid.New()
	code := "r20-missing-race"
	binding := ownershipJournalBindingFixture()
	binding.OperationID, binding.SpaceID = uuid.New(), space.ID
	binding.AccountID, binding.ActorProfileID, binding.NewOwnerProfileID = uuid.New(), owner, newOwner
	bindingBytes, bindingHash, err := EncodeOwnershipBinding(binding)
	require.NoError(t, err)

	_, err = st.Pool.Exec(ctx, `ALTER TABLE invites RENAME TO invites_r20_backing`)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION r20_pause_missing_resolver() RETURNS boolean LANGUAGE plpgsql VOLATILE AS $$
		BEGIN PERFORM pg_advisory_xact_lock(72020023); RETURN false; END $$`)
	require.NoError(t, err)
	viewSQL := fmt.Sprintf(`CREATE VIEW invites AS
		SELECT id,space_id,code,creator_profile_id,max_uses,use_count,expires_at,created_at,revoked_at FROM invites_r20_backing
		UNION ALL SELECT '%s'::uuid,'%s'::uuid,'%s'::varchar(32),'%s'::uuid,NULL::integer,0::integer,
		NULL::timestamptz,now(),NULL::timestamptz WHERE r20_pause_missing_resolver()`,
		inviteID, space.ID, "r20-missing-gate", owner)
	_, err = st.Pool.Exec(ctx, viewSQL)
	require.NoError(t, err)

	barrier, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = barrier.Rollback(context.Background()) }()
	_, err = barrier.Exec(ctx, `SELECT pg_advisory_xact_lock(72020023)`)
	require.NoError(t, err)
	applicationName := "r20_missing_snapshot"
	workerPool := r22SecondSpacePool(t, ctx, st.Pool, applicationName)
	result := make(chan error, 1)
	go func() {
		result <- (&SpaceStore{Pool: workerPool}).RevokeInvite(ctx, inviteID, owner)
	}()
	waitR20NamedPoolLock(t, ctx, st.Pool, applicationName)

	creator, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = creator.Rollback(context.Background()) }()
	_, err = creator.Exec(ctx, `INSERT INTO invites_r20_backing(id,space_id,code,creator_profile_id)
		VALUES($1,$2,$3,$4)`, inviteID, space.ID, code, owner)
	require.NoError(t, err)
	_, err = creator.Exec(ctx, `INSERT INTO ownership_journal(
		operation_id,protocol_version,space_id,account_id,actor_profile_id,new_owner_profile_id,
		session_epoch,proof_digest,binding_bytes,binding_hash,audit_id,event_id,state)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,'reserved')`,
		binding.OperationID, binding.ProtocolVersion, binding.SpaceID, binding.AccountID,
		binding.ActorProfileID, binding.NewOwnerProfileID, binding.SessionEpoch, binding.ProofDigest,
		bindingBytes, bindingHash[:], uuid.New(), uuid.New())
	require.NoError(t, err)
	require.NoError(t, creator.Commit(ctx))
	require.NoError(t, barrier.Commit(ctx))

	callErr := <-result
	require.ErrorIs(t, callErr, ErrInviteNotFound, "the first proven-missing result is the operation snapshot")
	require.NoError(t, ctx.Err())
	var inviteCount int
	var revokedAt *time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*),max(revoked_at) FROM invites_r20_backing WHERE id=$1`, inviteID).Scan(&inviteCount, &revokedAt))
	require.Equal(t, 1, inviteCount)
	require.Nil(t, revokedAt, "the original missing call must never act on the concurrently created invite")
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&state))
	require.Equal(t, "reserved", state)
}

func TestOwnershipScope_DynamicAndExplicitMultiSpaceSetsFailAtomically(t *testing.T) {
	st := ownershipScopeStoreFixture(t)
	pending := newR20ScopeFixture(t, st, "reserved")
	normal := newR20ScopeFixture(t, st, "")
	for _, profile := range []uuid.UUID{pending.member, normal.member} {
		_, err := st.Pool.Exec(context.Background(), `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, normal.binding.SpaceID, profile)
		require.NoError(t, err)
	}
	beforePending := r20ScopeSnapshot(t, st, pending.binding.SpaceID)
	beforeNormal := r20ScopeSnapshot(t, st, normal.binding.SpaceID)
	_, err := st.AreCoMembers(context.Background(), pending.owner, pending.member, []uuid.UUID{normal.binding.SpaceID, pending.binding.SpaceID})
	require.Error(t, err)
	_, err = st.AreCoMembers(context.Background(), pending.owner, pending.member, nil)
	require.Error(t, err)
	_, err = st.ListMySpacesPage(context.Background(), pending.member, "", 10)
	require.Error(t, err)
	require.Equal(t, beforePending, r20ScopeSnapshot(t, st, pending.binding.SpaceID))
	require.Equal(t, beforeNormal, r20ScopeSnapshot(t, st, normal.binding.SpaceID))
}

func TestOwnershipScope_EmptySameProfileIdentityDoesNotClaimUnrelatedSpaces(t *testing.T) {
	st := ownershipScopeStoreFixture(t)
	_ = newR20ScopeFixture(t, st, "reserved")
	profile := uuid.New()
	coMembers, err := st.AreCoMembers(context.Background(), profile, profile, nil)
	require.NoError(t, err)
	require.True(t, coMembers)
}

func waitR20NamedPoolLock(t *testing.T, ctx context.Context, observer *pgxpool.Pool, applicationName string) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		require.NoError(t, observer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND state='active' AND wait_event_type='Lock')`, applicationName).Scan(&waiting))
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("%s did not reach deterministic lock barrier: %v", applicationName, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitR20NamedPoolBlockedByPID(t *testing.T, ctx context.Context, observer *pgxpool.Pool, applicationName string, blockerPID int32) int32 {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var workerPID int32
		require.NoError(t, observer.QueryRow(ctx, `SELECT COALESCE((
			SELECT a.pid FROM pg_stat_activity a
			JOIN pg_locks waiting ON waiting.pid=a.pid AND waiting.locktype='advisory' AND NOT waiting.granted
			JOIN pg_locks held ON held.pid=$2 AND held.locktype='advisory' AND held.granted
				AND held.database IS NOT DISTINCT FROM waiting.database
				AND held.classid=waiting.classid AND held.objid=waiting.objid AND held.objsubid=waiting.objsubid
			WHERE a.application_name=$1 AND a.state='active' AND a.wait_event_type='Lock'
			LIMIT 1
		),0)`, applicationName, blockerPID).Scan(&workerPID))
		if workerPID != 0 {
			return workerPID
		}
		select {
		case <-ctx.Done():
			t.Fatalf("%s did not wait for blocker pid %d: %v", applicationName, blockerPID, ctx.Err())
		case <-ticker.C:
		}
	}
}

func requireR20WorkerReleasedAdvisoryLock(t *testing.T, ctx context.Context, observer *pgxpool.Pool, workerPID int32, classID, objectID int64, objectSubID int32) {
	t.Helper()
	var held bool
	require.NoError(t, observer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks
		WHERE pid=$1 AND locktype='advisory' AND granted
		AND classid::bigint=$2 AND objid::bigint=$3 AND objsubid=$4)`, workerPID, classID, objectID, objectSubID).Scan(&held))
	require.False(t, held, "candidate-set drift must roll back and release Space A before the retry waits on lower UUID Space B")
}

type ownershipFrozenErrorSource interface {
	OwnershipFrozenError() error
}

func requireR20OwnershipFrozenWithLiveContext(t *testing.T, ctx context.Context, st *SpaceStore, err error) {
	t.Helper()
	source, ok := any(st).(ownershipFrozenErrorSource)
	require.True(t, ok, "SpaceStore must expose the canonical ownership-frozen sentinel")
	target := source.OwnershipFrozenError()
	require.Error(t, target)
	require.ErrorIs(t, err, target)
	require.NoError(t, ctx.Err(), "ownership freeze must be decided before the caller deadline")
}

func TestOwnershipScope_DynamicCandidateSetDriftRetriesThenDeniesNewPendingSpace(t *testing.T) {
	for _, api := range []string{"AreCoMembers", "ListMySpacesPage"} {
		t.Run(api, func(t *testing.T) {
			st := ownershipScopeStoreFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			profileA, profileB := uuid.New(), uuid.New()
			spaceA, err := st.CreateSpace(ctx, profileA, "candidate A", "", "private")
			require.NoError(t, err)
			spaceB, err := st.CreateSpace(ctx, profileA, "candidate B", "", "private")
			require.NoError(t, err)
			if bytes.Compare(spaceA.ID[:], spaceB.ID[:]) < 0 {
				spaceA, spaceB = spaceB, spaceA
			}
			require.Positive(t, bytes.Compare(spaceA.ID[:], spaceB.ID[:]), "semantic fixture requires A=max(UUID), B=min(UUID)")
			_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, spaceA.ID, profileB)
			require.NoError(t, err)
			binding := ownershipJournalBindingFixture()
			binding.SpaceID, binding.ActorProfileID, binding.NewOwnerProfileID = spaceB.ID, profileA, profileB
			binding.OperationID, binding.AccountID = uuid.New(), uuid.New()

			blockerA, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = blockerA.Rollback(context.Background()) }()
			_, err = blockerA.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(spaceA.ID))
			require.NoError(t, err)
			var blockerAPID int32
			var lockAClassID, lockAObjectID int64
			var lockAObjectSubID int32
			require.NoError(t, blockerA.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerAPID))
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT classid::bigint,objid::bigint,objsubid FROM pg_locks
				WHERE pid=$1 AND locktype='advisory' AND granted`, blockerAPID).Scan(&lockAClassID, &lockAObjectID, &lockAObjectSubID))
			applicationName := "r20_dynamic_" + api
			workerPool := r22SecondSpacePool(t, ctx, st.Pool, applicationName)
			result := make(chan error, 1)
			go func() {
				worker := &SpaceStore{Pool: workerPool}
				if api == "AreCoMembers" {
					_, callErr := worker.AreCoMembers(ctx, profileA, profileB, nil)
					result <- callErr
					return
				}
				_, callErr := worker.ListMySpacesPage(ctx, profileB, "", 10)
				result <- callErr
			}()
			waitR20NamedPoolLock(t, ctx, st.Pool, applicationName)
			_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, spaceB.ID, profileB)
			require.NoError(t, err)
			_, err = st.ReserveOwnership(ctx, binding)
			require.NoError(t, err)

			blockerB, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = blockerB.Rollback(context.Background()) }()
			_, err = blockerB.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(spaceB.ID))
			require.NoError(t, err)
			var blockerBPID int32
			require.NoError(t, blockerB.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerBPID))
			require.NoError(t, blockerA.Commit(ctx))
			workerPID := waitR20NamedPoolBlockedByPID(t, ctx, st.Pool, applicationName, blockerBPID)
			requireR20WorkerReleasedAdvisoryLock(t, ctx, st.Pool, workerPID, lockAClassID, lockAObjectID, lockAObjectSubID)
			require.NoError(t, blockerB.Commit(ctx))
			requireR20OwnershipFrozenWithLiveContext(t, ctx, st, <-result)
		})
	}
}

func TestOwnershipScope_DynamicCandidateSetDriftRetriesLocksSecondSpaceAndReturnsRevalidatedAnswer(t *testing.T) {
	for _, api := range []string{"AreCoMembers", "ListMySpacesPage"} {
		t.Run(api, func(t *testing.T) {
			st := ownershipScopeStoreFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			profileA, profileB := uuid.New(), uuid.New()
			spaceA, err := st.CreateSpace(ctx, profileA, "candidate A", "", "private")
			require.NoError(t, err)
			spaceB, err := st.CreateSpace(ctx, profileA, "candidate B", "", "private")
			require.NoError(t, err)
			if bytes.Compare(spaceA.ID[:], spaceB.ID[:]) < 0 {
				spaceA, spaceB = spaceB, spaceA
			}
			require.Positive(t, bytes.Compare(spaceA.ID[:], spaceB.ID[:]), "semantic fixture requires A=max(UUID), B=min(UUID)")
			_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id,joined_at) VALUES($1,$2,$3)`, spaceA.ID, profileB, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
			require.NoError(t, err)

			blockerA, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = blockerA.Rollback(context.Background()) }()
			_, err = blockerA.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(spaceA.ID))
			require.NoError(t, err)
			var blockerAPID int32
			var lockAClassID, lockAObjectID int64
			var lockAObjectSubID int32
			require.NoError(t, blockerA.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerAPID))
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT classid::bigint,objid::bigint,objsubid FROM pg_locks
				WHERE pid=$1 AND locktype='advisory' AND granted`, blockerAPID).Scan(&lockAClassID, &lockAObjectID, &lockAObjectSubID))
			applicationName := "r20_dynamic_success_" + api
			workerPool := r22SecondSpacePool(t, ctx, st.Pool, applicationName)
			type dynamicResult struct {
				coMembers bool
				page      *ListMySpacesPage
				err       error
			}
			result := make(chan dynamicResult, 1)
			go func() {
				worker := &SpaceStore{Pool: workerPool}
				if api == "AreCoMembers" {
					value, callErr := worker.AreCoMembers(ctx, profileA, profileB, nil)
					result <- dynamicResult{coMembers: value, err: callErr}
					return
				}
				page, callErr := worker.ListMySpacesPage(ctx, profileB, "", 10)
				result <- dynamicResult{page: page, err: callErr}
			}()
			waitR20NamedPoolLock(t, ctx, st.Pool, applicationName)
			_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id,joined_at) VALUES($1,$2,$3)`, spaceB.ID, profileB, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
			require.NoError(t, err)

			blockerB, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = blockerB.Rollback(context.Background()) }()
			_, err = blockerB.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(spaceB.ID))
			require.NoError(t, err)
			var blockerBPID int32
			require.NoError(t, blockerB.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerBPID))
			require.NoError(t, blockerA.Commit(ctx))
			workerPID := waitR20NamedPoolBlockedByPID(t, ctx, st.Pool, applicationName, blockerBPID)
			requireR20WorkerReleasedAdvisoryLock(t, ctx, st.Pool, workerPID, lockAClassID, lockAObjectID, lockAObjectSubID)
			require.NoError(t, blockerB.Commit(ctx))

			got := <-result
			require.NoError(t, got.err)
			require.NoError(t, ctx.Err())
			if api == "AreCoMembers" {
				require.True(t, got.coMembers)
				return
			}
			require.Len(t, got.page.Rows, 2)
			require.Equal(t, []uuid.UUID{spaceB.ID, spaceA.ID}, []uuid.UUID{got.page.Rows[0].ID, got.page.Rows[1].ID}, "page order must be computed after the equal-set revalidation")
			require.Empty(t, got.page.NextCursor)
		})
	}
}

func TestOwnershipScope_ReversedExplicitSpaceOrderCannotDeadlock(t *testing.T) {
	st := ownershipScopeStoreFixture(t)
	a := newR20ScopeFixture(t, st, "reserved")
	b := newR20ScopeFixture(t, st, "reserved")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := make(chan struct{})
	result := make(chan error, 2)
	for _, ids := range [][]uuid.UUID{{a.binding.SpaceID, b.binding.SpaceID}, {b.binding.SpaceID, a.binding.SpaceID}} {
		ids := append([]uuid.UUID(nil), ids...)
		go func() {
			<-start
			_, err := st.AreCoMembers(ctx, a.owner, a.member, ids)
			result <- err
		}()
	}
	close(start)
	for i := 0; i < 2; i++ {
		select {
		case err := <-result:
			require.Error(t, err)
		case <-ctx.Done():
			t.Fatalf("reversed space order deadlocked: %v", ctx.Err())
		}
	}
}

func TestOwnershipScope_JournalLookupOutageAndCancelledLockFailClosed(t *testing.T) {
	st := ownershipScopeStoreFixture(t)
	f := newR20ScopeFixture(t, st, "reserved")
	_, err := st.Pool.Exec(context.Background(), `ALTER TABLE ownership_journal RENAME TO ownership_journal_unavailable`)
	require.NoError(t, err)
	_, err = st.GetSpace(context.Background(), f.binding.SpaceID)
	require.Error(t, err)
	_, err = st.Pool.Exec(context.Background(), `ALTER TABLE ownership_journal_unavailable RENAME TO ownership_journal`)
	require.NoError(t, err)

	blocker, err := st.Pool.Begin(context.Background())
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(context.Background(), `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(f.binding.SpaceID))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = st.GetSpace(ctx, f.binding.SpaceID)
	require.Error(t, err)
}

func TestOwnershipScope_OrdinaryActionAndReservationLinearizeOnOneSpaceLock(t *testing.T) {
	st := ownershipScopeStoreFixture(t)
	f := newR20ScopeFixture(t, st, "")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	blocker, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, spaceMutationAdvisoryKey(f.binding.SpaceID))
	require.NoError(t, err)

	ordinaryPool := r22SecondSpacePool(t, ctx, st.Pool, "r20_ordinary_linearize")
	ordinaryResult := make(chan error, 1)
	go func() {
		_, callErr := (&SpaceStore{Pool: ordinaryPool}).GetSpace(ctx, f.binding.SpaceID)
		ordinaryResult <- callErr
	}()
	waitR20NamedPoolLock(t, ctx, st.Pool, "r20_ordinary_linearize")
	reservationPool := r22SecondSpacePool(t, ctx, st.Pool, "r20_reservation_linearize")
	reserveResult := make(chan error, 1)
	go func() {
		_, callErr := (&SpaceStore{Pool: reservationPool}).ReserveOwnership(ctx, f.binding)
		reserveResult <- callErr
	}()
	waitR20NamedPoolLock(t, ctx, st.Pool, "r20_reservation_linearize")
	require.NoError(t, blocker.Commit(ctx))

	first := <-ordinaryResult
	second := <-reserveResult
	if first == nil {
		require.NoError(t, second)
		_, err = st.GetSpace(ctx, f.binding.SpaceID)
		require.Error(t, err, "once reservation owns the scope, a new ordinary read must fail")
		return
	}
	require.NoError(t, second)
	require.Error(t, first)
}

func r20WorkerHoldsAdvisoryLock(t *testing.T, ctx context.Context, observer *pgxpool.Pool, workerPID int32, classID, objectID int64, objectSubID int32) bool {
	t.Helper()
	var held bool
	require.NoError(t, observer.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks
		WHERE pid=$1 AND locktype='advisory' AND granted
		AND classid::bigint=$2 AND objid::bigint=$3 AND objsubid=$4)`, workerPID, classID, objectID, objectSubID).Scan(&held))
	return held
}

func TestOwnershipScope_DirectAndIndirectActionsShareTheCheckedTransaction(t *testing.T) {
	for _, indirect := range []bool{false, true} {
		name := "direct UpdateSpace"
		if indirect {
			name = "indirect RevokeInvite"
		}
		t.Run(name, func(t *testing.T) {
			st := ownershipScopeStoreFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			owner, newOwner := uuid.New(), uuid.New()
			space, err := st.CreateSpace(ctx, owner, "same transaction", "", "private")
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, newOwner)
			require.NoError(t, err)
			binding := ownershipJournalBindingFixture()
			binding.OperationID, binding.SpaceID = uuid.New(), space.ID
			binding.AccountID, binding.ActorProfileID, binding.NewOwnerProfileID = uuid.New(), owner, newOwner

			var invite *InviteRow
			table := "spaces"
			if indirect {
				invite, err = st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
				require.NoError(t, err)
				table = "invites"
			}
			_, err = st.Pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION r20_pause_ordinary_action() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN PERFORM pg_advisory_xact_lock(72020020); RETURN NEW; END $$;
				CREATE TRIGGER r20_pause_ordinary_action BEFORE UPDATE ON %s
				FOR EACH ROW EXECUTE FUNCTION r20_pause_ordinary_action()`, table))
			require.NoError(t, err)

			barrier, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = barrier.Rollback(context.Background()) }()
			_, err = barrier.Exec(ctx, `SELECT pg_advisory_xact_lock(72020020)`)
			require.NoError(t, err)
			var barrierPID int32
			require.NoError(t, barrier.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&barrierPID))

			scopeBlocker, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = scopeBlocker.Rollback(context.Background()) }()
			_, err = scopeBlocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(space.ID))
			require.NoError(t, err)
			var scopeBlockerPID int32
			var classID, objectID int64
			var objectSubID int32
			require.NoError(t, scopeBlocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&scopeBlockerPID))
			require.NoError(t, scopeBlocker.QueryRow(ctx, `SELECT classid::bigint,objid::bigint,objsubid FROM pg_locks
				WHERE pid=pg_backend_pid() AND locktype='advisory' AND granted`).Scan(&classID, &objectID, &objectSubID))

			actionName := "r20_same_tx_action"
			actionPool := r22SecondSpacePool(t, ctx, st.Pool, actionName)
			actionResult := make(chan error, 1)
			go func() {
				worker := &SpaceStore{Pool: actionPool}
				if indirect {
					actionResult <- worker.RevokeInvite(ctx, invite.ID, owner)
					return
				}
				updatedName := "updated before reservation"
				_, callErr := worker.UpdateSpace(ctx, space.ID, UpdateSpaceInput{Name: &updatedName})
				actionResult <- callErr
			}()
			waitR20NamedPoolBlockedByPID(t, ctx, st.Pool, actionName, scopeBlockerPID)

			reservationName := "r20_same_tx_reservation"
			reservationPool := r22SecondSpacePool(t, ctx, st.Pool, reservationName)
			reservationResult := make(chan error, 1)
			go func() {
				_, reserveErr := (&SpaceStore{Pool: reservationPool}).ReserveOwnership(ctx, binding)
				reservationResult <- reserveErr
			}()
			waitR20NamedPoolBlockedByPID(t, ctx, st.Pool, reservationName, scopeBlockerPID)
			require.NoError(t, scopeBlocker.Commit(ctx))

			actionPID := waitR20NamedPoolBlockedByPID(t, ctx, st.Pool, actionName, barrierPID)
			holdsSpaceLock := r20WorkerHoldsAdvisoryLock(t, ctx, st.Pool, actionPID, classID, objectID, objectSubID)
			reservationCompleted := false
			var reservationErr error
			select {
			case reservationErr = <-reservationResult:
				reservationCompleted = true
			default:
			}
			if holdsSpaceLock {
				waitR20NamedPoolBlockedByPID(t, ctx, st.Pool, reservationName, actionPID)
			}
			require.NoError(t, barrier.Commit(ctx))
			require.NoError(t, <-actionResult)
			if !reservationCompleted {
				reservationErr = <-reservationResult
			}
			require.NoError(t, reservationErr)
			require.False(t, reservationCompleted, "reservation passed between the ownership check and ordinary SQL")
			require.True(t, holdsSpaceLock, "ordinary SQL backend must retain the checked Space advisory lock")
			_, err = st.GetSpace(ctx, space.ID)
			require.ErrorIs(t, err, ErrOwnershipFrozen)
		})
	}
}

func TestOwnershipScope_CheckRunsAfterTheActionTransactionTakesTheSpaceLock(t *testing.T) {
	for _, indirect := range []bool{false, true} {
		name := "direct UpdateSpace"
		if indirect {
			name = "indirect RevokeInvite"
		}
		t.Run(name, func(t *testing.T) {
			st := ownershipScopeStoreFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			owner, newOwner := uuid.New(), uuid.New()
			space, err := st.CreateSpace(ctx, owner, "check after lock", "", "private")
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, newOwner)
			require.NoError(t, err)
			binding := ownershipJournalBindingFixture()
			binding.OperationID, binding.SpaceID = uuid.New(), space.ID
			binding.AccountID, binding.ActorProfileID, binding.NewOwnerProfileID = uuid.New(), owner, newOwner
			bindingBytes, bindingHash, err := EncodeOwnershipBinding(binding)
			require.NoError(t, err)

			var invite *InviteRow
			table := "spaces"
			if indirect {
				invite, err = st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
				require.NoError(t, err)
				table = "invites"
			}
			_, err = st.Pool.Exec(ctx, fmt.Sprintf(`CREATE SEQUENCE r20_dml_probe;
				CREATE FUNCTION r20_mark_ordinary_dml() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN PERFORM nextval('r20_dml_probe'); RETURN NEW; END $$;
				CREATE TRIGGER r20_mark_ordinary_dml BEFORE UPDATE ON %s
				FOR EACH ROW EXECUTE FUNCTION r20_mark_ordinary_dml()`, table))
			require.NoError(t, err)

			blocker, err := st.Pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = blocker.Rollback(context.Background()) }()
			_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(space.ID))
			require.NoError(t, err)
			var blockerPID int32
			require.NoError(t, blocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID))

			actionName := "r20_check_after_lock"
			actionPool := r22SecondSpacePool(t, ctx, st.Pool, actionName)
			actionResult := make(chan error, 1)
			go func() {
				worker := &SpaceStore{Pool: actionPool}
				if indirect {
					actionResult <- worker.RevokeInvite(ctx, invite.ID, owner)
					return
				}
				updatedName := "must not update"
				_, callErr := worker.UpdateSpace(ctx, space.ID, UpdateSpaceInput{Name: &updatedName})
				actionResult <- callErr
			}()
			waitR20NamedPoolBlockedByPID(t, ctx, st.Pool, actionName, blockerPID)
			_, err = blocker.Exec(ctx, `INSERT INTO ownership_journal(
				operation_id,protocol_version,space_id,account_id,actor_profile_id,new_owner_profile_id,
				session_epoch,proof_digest,binding_bytes,binding_hash,state,audit_id,event_id)
				VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'reserved',$11,$12)`,
				binding.OperationID, binding.ProtocolVersion, binding.SpaceID, binding.AccountID,
				binding.ActorProfileID, binding.NewOwnerProfileID, binding.SessionEpoch, binding.ProofDigest,
				bindingBytes, bindingHash[:], uuid.New(), uuid.New())
			require.NoError(t, err)
			require.NoError(t, blocker.Commit(ctx))

			requireR20OwnershipFrozenWithLiveContext(t, ctx, st, <-actionResult)
			var dmlCalled bool
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT is_called FROM r20_dml_probe`).Scan(&dmlCalled))
			require.False(t, dmlCalled, "journal check must reject before ordinary DML")
		})
	}
}

func TestOwnershipScope_InventoryNamesRemainExact(t *testing.T) {
	seen := map[string]bool{}
	for _, tc := range r20OrdinaryScopeCases() {
		require.False(t, seen[tc.name])
		seen[tc.name] = true
	}
	for _, requiredName := range []string{"GetSpace", "ListMySpacesPage", "AreCoMembers", "JoinByInvite", "AddBotMember", "MemberCap", "ListAuditLogSignedPage", "ListSpaceTree", "ResolveVoiceRoomAccess"} {
		require.True(t, seen[requiredName], fmt.Sprintf("canonical ordinary inventory lost %s", requiredName))
	}
}
