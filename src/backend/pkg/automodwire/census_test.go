package automodwire

import (
	"encoding/hex"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
	automodv1 "voice/backend/pkg/pb/voice/automod/v1"
)

func censusVector() *automodv1.ResolveModerationSubjectResponse {
	return &automodv1.ResolveModerationSubjectResponse{Subject: &automodv1.SubjectBinding{ProtocolVersion: 1, AccountId: "00000000-0000-4000-8000-000000000001", SubjectId: "80000000-0000-4000-8000-000000000002", LifecycleRevision: 3, State: automodv1.SubjectState_SUBJECT_STATE_LIVE}, SessionEpoch: 7}
}
func TestCensusDigest_MatchesIndependentlyFramedJavaVector(t *testing.T) {
	digest, err := CensusDigest([]*automodv1.ResolveModerationSubjectResponse{censusVector()})
	require.NoError(t, err)
	require.Equal(t, "2e75915e1a536d1db68e0e35f601528e79b9829aab8b0cc61ad1eea30d872571", hex.EncodeToString(digest[:]))
	body, err := proto.Marshal(censusVector())
	require.NoError(t, err)
	require.Len(t, body, 86)
}

func TestAccountCensus_StrictCompleteViewAndRetainedDeadline(t *testing.T) {
	digest, err := CensusDigest([]*automodv1.ResolveModerationSubjectResponse{censusVector()})
	require.NoError(t, err)
	valid := &automodv1.ListModerationAccountCensusResponse{Accounts: []*automodv1.ResolveModerationSubjectResponse{censusVector()}, Complete: true, Seal: &automodv1.ModerationAuthoritySnapshotSeal{ProtocolVersion: 1, AuthorityGeneration: uuid.NewString(), SnapshotId: uuid.NewString(), Cut: timestamppb.New(time.Now().UTC().Truncate(time.Microsecond)), ItemCount: 1, TerminalHighWater: 99, CanonicalSha256: digest[:], Complete: true}}
	require.NoError(t, ValidateAccountCensus(valid), "terminal high-water is independent from live census size")
	for _, defect := range []string{"bare-complete", "partial", "seal-partial", "duplicate", "subject", "epoch", "count", "hash", "unknown", "cut", "generation", "purged", "oversize"} {
		t.Run(defect, func(t *testing.T) {
			bad := proto.Clone(valid).(*automodv1.ListModerationAccountCensusResponse)
			switch defect {
			case "bare-complete":
				bad.Seal = nil
			case "partial":
				bad.Complete = false
			case "seal-partial":
				bad.Seal.Complete = false
			case "duplicate":
				bad.Accounts = append(bad.Accounts, proto.Clone(bad.Accounts[0]).(*automodv1.ResolveModerationSubjectResponse))
				bad.Seal.ItemCount = 2
			case "subject":
				bad.Accounts[0].Subject.SubjectId = "raw-account"
			case "epoch":
				bad.Accounts[0].SessionEpoch = 0
			case "count":
				bad.Seal.ItemCount++
			case "hash":
				bad.Seal.CanonicalSha256 = make([]byte, 32)
			case "unknown":
				bad.Accounts[0].Subject.ProtoReflect().SetUnknown([]byte{0x80, 0x06, 0x01})
			case "cut":
				bad.Seal.Cut.Nanos++
			case "generation":
				bad.Seal.AuthorityGeneration = ""
			case "purged":
				bad.Accounts[0].Subject.State = automodv1.SubjectState_SUBJECT_STATE_PURGED
			case "oversize":
				bad.Accounts = make([]*automodv1.ResolveModerationSubjectResponse, MaxCensusAccounts+1)
				bad.Seal.ItemCount = uint64(len(bad.Accounts))
			}
			require.ErrorIs(t, ValidateAccountCensus(bad), ErrCore)
		})
	}
	retained := proto.Clone(valid).(*automodv1.ListModerationAccountCensusResponse)
	retained.Accounts[0].Subject.State = automodv1.SubjectState_SUBJECT_STATE_DELETED
	cycle := uuid.NewString()
	retained.Accounts[0].Subject.DeletionCycleId = &cycle
	retained.Accounts[0].Subject.PurgeAt = timestamppb.New(valid.Seal.Cut.AsTime().Add(time.Hour))
	retained.Accounts[0].AuthRestricted = true
	digest, err = CensusDigest(retained.Accounts)
	require.NoError(t, err)
	retained.Seal.CanonicalSha256 = digest[:]
	require.NoError(t, ValidateAccountCensus(retained))
	retained.Accounts[0].Subject.PurgeAt = proto.Clone(valid.Seal.Cut).(*timestamppb.Timestamp)
	digest, err = CensusDigest(retained.Accounts)
	require.NoError(t, err)
	retained.Seal.CanonicalSha256 = digest[:]
	require.ErrorIs(t, ValidateAccountCensus(retained), ErrCore, "deadline equality is already due")
}
