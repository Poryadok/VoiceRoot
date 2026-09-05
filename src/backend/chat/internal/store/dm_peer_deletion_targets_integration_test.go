package store

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type dmPeerDeletionTargetForTest struct {
	ChatID             uuid.UUID
	SurvivingProfileID uuid.UUID
}

// callDMPeerDeletionTargets keeps this RED test independent of the production
// result type while still requiring the canonical DMStore method and its
// public ChatID/SurvivingProfileID contract.
func callDMPeerDeletionTargets(
	t *testing.T, ctx context.Context, dm *DMStore, deletedProfileIDs []uuid.UUID,
) []dmPeerDeletionTargetForTest {
	t.Helper()

	method := reflect.ValueOf(dm).MethodByName("ListDMPeerDeletionTargets")
	require.True(t, method.IsValid(), "DMStore must expose ListDMPeerDeletionTargets")
	results := method.Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(deletedProfileIDs)})
	require.Len(t, results, 2, "ListDMPeerDeletionTargets must return (targets, error)")
	if !results[1].IsNil() {
		require.NoError(t, results[1].Interface().(error))
	}

	targets := results[0]
	if targets.Kind() == reflect.Pointer {
		targets = targets.Elem()
	}
	require.Equal(t, reflect.Slice, targets.Kind(), "targets must be a slice")

	out := make([]dmPeerDeletionTargetForTest, 0, targets.Len())
	for i := 0; i < targets.Len(); i++ {
		item := targets.Index(i)
		if item.Kind() == reflect.Pointer {
			item = item.Elem()
		}
		require.Equal(t, reflect.Struct, item.Kind(), "target must be a struct")
		chatID := item.FieldByName("ChatID")
		survivorID := item.FieldByName("SurvivingProfileID")
		require.True(t, chatID.IsValid(), "target must expose ChatID")
		require.True(t, survivorID.IsValid(), "target must expose SurvivingProfileID")
		require.Equal(t, reflect.TypeOf(uuid.UUID{}), chatID.Type())
		require.Equal(t, reflect.TypeOf(uuid.UUID{}), survivorID.Type())
		publicFields := make([]string, 0, item.NumField())
		for fieldIndex := 0; fieldIndex < item.NumField(); fieldIndex++ {
			field := item.Type().Field(fieldIndex)
			if field.PkgPath == "" {
				publicFields = append(publicFields, field.Name)
			}
		}
		require.ElementsMatch(t, []string{"ChatID", "SurvivingProfileID"}, publicFields,
			"target must not expose deleted-account/profile identity")
		for _, fieldName := range publicFields {
			lower := strings.ToLower(fieldName)
			require.NotContains(t, lower, "deleted")
			require.NotContains(t, lower, "account")
		}
		out = append(out, dmPeerDeletionTargetForTest{
			ChatID:             chatID.Interface().(uuid.UUID),
			SurvivingProfileID: survivorID.Interface().(uuid.UUID),
		})
	}
	return out
}

func requireDMPeerDeletionTargetMethod(t *testing.T) {
	t.Helper()
	_, ok := reflect.TypeOf(&DMStore{}).MethodByName("ListDMPeerDeletionTargets")
	require.True(t, ok, "DMStore must expose ListDMPeerDeletionTargets")
}

func sortDMPeerDeletionTargets(targets []dmPeerDeletionTargetForTest) {
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].ChatID != targets[j].ChatID {
			return targets[i].ChatID.String() < targets[j].ChatID.String()
		}
		return targets[i].SurvivingProfileID.String() < targets[j].SurvivingProfileID.String()
	})
}

// TestListDMPeerDeletionTargets_OnlyDirectDMsWithExactlyOneSurvivor documents
// the account-delete fanout target boundary. The supplied profile set stands
// for all profiles resolved from one deleted account.
func TestListDMPeerDeletionTargets_OnlyDirectDMsWithExactlyOneSurvivor(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	requireDMPeerDeletionTargetMethod(t)
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	dm := &DMStore{Pool: pool}

	deletedProfileA := uuid.New()
	deletedProfileB := uuid.New()
	survivorA := uuid.New()
	survivorB := uuid.New()

	dmA, _, err := dm.EnsureDM(ctx, survivorA, deletedProfileA, InboxMain)
	require.NoError(t, err)
	dmB, _, err := dm.EnsureDM(ctx, survivorB, deletedProfileB, InboxMain)
	require.NoError(t, err)
	_, _, err = dm.EnsureDM(ctx, deletedProfileA, deletedProfileB, InboxMain)
	require.NoError(t, err, "a DM with two deleted members is a valid corruption fixture")

	group, err := dm.CreateGroupChat(ctx, deletedProfileA, "deleted-peer-group", nil)
	require.NoError(t, err)
	_, err = dm.AddGroupMembers(WithGroupMinMembers(ctx, 2), group.ID, []uuid.UUID{survivorA})
	require.NoError(t, err)

	channel, err := dm.CreateChannelChat(ctx, deletedProfileB, "deleted-peer-channel", nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket)
VALUES ($1, $2, 'member', 'main')
`, channel.ID, survivorB)
	require.NoError(t, err)

	got := callDMPeerDeletionTargets(
		t,
		ctx,
		dm,
		[]uuid.UUID{deletedProfileB, deletedProfileA},
	)

	expected := []dmPeerDeletionTargetForTest{
		{ChatID: dmA.ID, SurvivingProfileID: survivorA},
		{ChatID: dmB.ID, SurvivingProfileID: survivorB},
	}
	sortDMPeerDeletionTargets(expected)
	require.Equal(t, expected, got)
	for _, target := range got {
		require.NotContains(t, []uuid.UUID{deletedProfileA, deletedProfileB}, target.SurvivingProfileID)
	}
}

// TestListDMPeerDeletionTargets_DuplicateUnorderedProfilesReturnStableUniqueRows
// requires the same canonical order for duplicate/unordered account profile
// inputs. It also prevents duplicate live fanout targets.
func TestListDMPeerDeletionTargets_DuplicateUnorderedProfilesReturnStableUniqueRows(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	requireDMPeerDeletionTargetMethod(t)
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	dm := &DMStore{Pool: pool}

	deletedProfileA := uuid.New()
	deletedProfileB := uuid.New()
	survivorA := uuid.New()
	survivorB := uuid.New()
	dmA, _, err := dm.EnsureDM(ctx, deletedProfileA, survivorA, InboxMain)
	require.NoError(t, err)
	dmB, _, err := dm.EnsureDM(ctx, deletedProfileB, survivorB, InboxMain)
	require.NoError(t, err)

	first := callDMPeerDeletionTargets(
		t,
		ctx,
		dm,
		[]uuid.UUID{deletedProfileB, deletedProfileA, deletedProfileB},
	)
	second := callDMPeerDeletionTargets(
		t,
		ctx,
		dm,
		[]uuid.UUID{deletedProfileA, deletedProfileB},
	)

	expected := []dmPeerDeletionTargetForTest{
		{ChatID: dmA.ID, SurvivingProfileID: survivorA},
		{ChatID: dmB.ID, SurvivingProfileID: survivorB},
	}
	sortDMPeerDeletionTargets(expected)
	require.Equal(t, expected, first)
	require.Equal(t, expected, second)
}
