package store

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math"

	"github.com/google/uuid"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/entrypolicy"
)

func entryPolicyPublicInput(request *spacev1.SetEntryPolicyRequest, account, profile uuid.UUID) (JoinOperationInput, entrypolicy.Policy, error) {
	if request == nil || account == uuid.Nil || profile == uuid.Nil || request.ExpectedPolicyVersion == 0 || request.ExpectedPolicyVersion > math.MaxInt64 || len(request.ProtoReflect().GetUnknown()) != 0 {
		return JoinOperationInput{}, entrypolicy.Policy{}, entrypolicy.ErrInvalid
	}
	space, err := canonicalJoinID(request.SpaceId)
	if err != nil {
		return JoinOperationInput{}, entrypolicy.Policy{}, err
	}
	operation, err := canonicalJoinID(request.OperationId)
	if err != nil {
		return JoinOperationInput{}, entrypolicy.Policy{}, err
	}
	policy, err := entrypolicy.Parse(request.PrivatePolicyJson)
	if err != nil {
		return JoinOperationInput{}, entrypolicy.Policy{}, err
	}
	raw, err := json.Marshal(policy)
	if err != nil {
		return JoinOperationInput{}, entrypolicy.Policy{}, entrypolicy.ErrInvalid
	}
	policySHA := sha256.Sum256(raw)
	method := spacev1.SpaceService_SetEntryPolicy_FullMethodName
	frame := []byte("voice.space.entry-policy-operation.v1\x00")
	frame = binary.BigEndian.AppendUint32(frame, 1)
	frame = binary.BigEndian.AppendUint32(frame, uint32(len(method)))
	frame = append(frame, method...)
	frame = append(frame, account[:]...)
	frame = append(frame, profile[:]...)
	frame = append(frame, space[:]...)
	frame = append(frame, operation[:]...)
	frame = binary.BigEndian.AppendUint64(frame, request.ExpectedPolicyVersion)
	frame = append(frame, policySHA[:]...)
	sha := sha256.Sum256(frame)
	return JoinOperationInput{SpaceID: space, ActorAccountID: account, ActorProfileID: profile, OperationID: operation, Method: method, RequestSHA256: sha[:]}, policy, nil
}
