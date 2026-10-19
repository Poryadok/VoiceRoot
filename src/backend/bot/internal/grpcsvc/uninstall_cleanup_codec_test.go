package grpcsvc

import (
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func uninstallCodecSource(t *testing.T) (*store.BotDeletion, store.BotDeletionEffect) {
	t.Helper()
	op, e := spaceDeletionSource(t)
	op.Purpose = "space_uninstall"
	var decision struct {
		OperationID, BotID, OwnerAccountID, ActorProfileID uuid.UUID
		Purpose                                            string
		Admission                                          *store.DeletionAdmission
	}
	require.NoError(t, json.Unmarshal(op.DecisionBytes, &decision))
	decision.Purpose = op.Purpose
	q := &botv1.UninstallBotFromSpaceRequest{BotId: op.BotID.String(), SpaceId: e.Target.SpaceID.String()}
	var err error
	decision.Admission.RequestBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	decision.Admission.RequestHash, err = principal.RequestHash(q)
	require.NoError(t, err)
	decision.Admission.Method = botv1.BotService_UninstallBotFromSpace_FullMethodName
	op.DecisionBytes, err = json.Marshal(decision)
	require.NoError(t, err)
	sum := sha256.Sum256(op.DecisionBytes)
	op.DecisionSHA256 = sum[:]
	return op, e
}

func TestUninstallPurposeAndLocalNativeACKCannotAcquireGlobalFence(t *testing.T) {
	op, e := uninstallCodecSource(t)
	roleEffect := e
	roleEffect.Kind = "ROLE_REMOVE"
	role, err := buildRoleDeletionRequest(op, roleEffect)
	require.NoError(t, err)
	require.Equal(t, "space_uninstall", role.Purpose)
	_, err = buildSpaceDeletionRequest(op, e)
	require.Error(t, err, "ordinary uninstall cannot construct global terminal authority")
	native, err := buildNativeCancellation(op, e)
	require.NoError(t, err)
	e.RequestBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(native)
	require.NoError(t, err)
	sum := sha256.Sum256(e.RequestBytes)
	observation := nativeCancellationObservation{1, "BOT_LOCAL_NATIVE_CANCEL_ACK", spacev1.SpaceService_RemoveBotMember_FullMethodName, sum[:], []byte{}, time.Now().UTC()}
	e.ReceiptBytes, err = json.Marshal(observation)
	require.NoError(t, err)
	require.NoError(t, validateCompletedDeletionEffect(op, e))
	observation.ResponseBytes = []byte("invented Space receipt")
	e.ReceiptBytes, err = json.Marshal(observation)
	require.NoError(t, err)
	require.Error(t, validateCompletedDeletionEffect(op, e))
	observation.ResponseBytes = []byte{}
	observation.Kind = "SPACE_GLOBAL_COMPLETED"
	e.ReceiptBytes, err = json.Marshal(observation)
	require.NoError(t, err)
	require.Error(t, validateCompletedDeletionEffect(op, e))
	op.Purpose = "global_delete"
	require.Error(t, validateCompletedDeletionEffect(op, e), "local observation cannot satisfy global cleanup")
	op.Purpose = "unknown-purpose"
	require.Error(t, validateDeletionSource(op, e))
}
