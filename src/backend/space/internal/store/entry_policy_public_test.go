package store

import (
	"bytes"
	"encoding/hex"
	"math"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
)

func TestEntryPolicyPublicBindingIndependentGoldenAndCanonicalPolicy(t *testing.T) {
	account := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	profile := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	request := &spacev1.SetEntryPolicyRequest{SpaceId: "33333333-3333-4333-8333-333333333333", OperationId: "44444444-4444-4444-8444-444444444444", ExpectedPolicyVersion: 7, PrivatePolicyJson: []byte(`{"schema_version":1,"require_phone":false,"require_captcha":false,"require_manual_approval":false,"questions":[]}`)}
	op, _, err := entryPolicyPublicInput(request, account, profile)
	require.NoError(t, err)
	require.Equal(t, "0c8c97122ccb35f6cbdf13d3d0c4d91f747f873c33be86e666883e227597e1f9", hex.EncodeToString(op.RequestSHA256), "independent Python fixed binary frame")
	require.False(t, validJoinOperation(op), "settings is not a join basis or generic join operation")
	equivalent := proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
	equivalent.PrivatePolicyJson = []byte(`{ "questions":[],"require_manual_approval":false,"require_captcha":false,"require_phone":false,"schema_version":1 }`)
	same, _, err := entryPolicyPublicInput(equivalent, account, profile)
	require.NoError(t, err)
	require.Equal(t, op.RequestSHA256, same.RequestSHA256)
	for name, change := range map[string]func(*spacev1.SetEntryPolicyRequest){
		"space":     func(r *spacev1.SetEntryPolicyRequest) { r.SpaceId = uuid.NewString() },
		"operation": func(r *spacev1.SetEntryPolicyRequest) { r.OperationId = uuid.NewString() },
		"version":   func(r *spacev1.SetEntryPolicyRequest) { r.ExpectedPolicyVersion++ },
		"policy": func(r *spacev1.SetEntryPolicyRequest) {
			r.PrivatePolicyJson = bytes.ReplaceAll(r.PrivatePolicyJson, []byte(`"require_manual_approval":false`), []byte(`"require_manual_approval":true`))
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
			change(changed)
			different, _, e := entryPolicyPublicInput(changed, account, profile)
			require.NoError(t, e)
			require.NotEqual(t, op.RequestSHA256, different.RequestSHA256)
		})
	}
	other, _, err := entryPolicyPublicInput(request, uuid.New(), profile)
	require.NoError(t, err)
	require.NotEqual(t, op.RequestSHA256, other.RequestSHA256)
	other, _, err = entryPolicyPublicInput(request, account, uuid.New())
	require.NoError(t, err)
	require.NotEqual(t, op.RequestSHA256, other.RequestSHA256)
	for _, version := range []uint64{0, math.MaxInt64 + 1} {
		invalid := proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
		invalid.ExpectedPolicyVersion = version
		_, _, err = entryPolicyPublicInput(invalid, account, profile)
		require.Error(t, err)
	}
	invalid := proto.Clone(request).(*spacev1.SetEntryPolicyRequest)
	invalid.PrivatePolicyJson = []byte(`{"schema_version":1,"schema_version":1}`)
	_, _, err = entryPolicyPublicInput(invalid, account, profile)
	require.Error(t, err)
}
