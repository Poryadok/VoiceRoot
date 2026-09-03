package grpcsvc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
)

// TestUpdateSpace_EntryQuestionsAndMmConfigRoundTrip guards the shipped JSON settings fields.
func TestUpdateSpace_EntryQuestionsAndMmConfigRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	ctx := context.Background()
	pool := startSpacePostgresForTest(t, ctx)
	applySpaceMigration(t, ctx, pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Settings round trip"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()
	questions := `[{"id":"rank","prompt":"Your rank?","required":true}]`
	mmConfig := `{"game_id":"cs2","region":"eu","verified_only":true}`

	updated, err := client.UpdateSpace(ownerCtx, &spacev1.UpdateSpaceRequest{
		SpaceId:            spaceID,
		EntryQuestionsJson: &questions,
		MmConfigJson:       &mmConfig,
	})
	require.NoError(t, err)
	require.JSONEq(t, questions, updated.GetSpace().GetEntryQuestionsJson())
	require.JSONEq(t, mmConfig, updated.GetSpace().GetMmConfigJson())

	got, err := client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.JSONEq(t, questions, got.GetSpace().GetEntryQuestionsJson())
	require.JSONEq(t, mmConfig, got.GetSpace().GetMmConfigJson())

	listed, err := client.ListMySpaces(ownerCtx, &spacev1.ListMySpacesRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, listed.GetSpaceList().GetSpaces(), 1)
	require.JSONEq(t, questions, listed.GetSpaceList().GetSpaces()[0].GetEntryQuestionsJson())
	require.JSONEq(t, mmConfig, listed.GetSpaceList().GetSpaces()[0].GetMmConfigJson())

	dedicatedConfig := `{"game_id":"dota2","region":"ru","team_size":5}`
	mmUpdated, err := client.UpdateSpaceMmConfig(ownerCtx, &spacev1.UpdateSpaceMmConfigRequest{
		SpaceId:      spaceID,
		MmConfigJson: dedicatedConfig,
	})
	require.NoError(t, err)
	require.JSONEq(t, questions, mmUpdated.GetSpace().GetEntryQuestionsJson())
	require.JSONEq(t, dedicatedConfig, mmUpdated.GetSpace().GetMmConfigJson())

	got, err = client.GetSpace(ownerCtx, &spacev1.GetSpaceRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.JSONEq(t, questions, got.GetSpace().GetEntryQuestionsJson())
	require.JSONEq(t, dedicatedConfig, got.GetSpace().GetMmConfigJson())
}
