package grpcsvc_test

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/grpcsvc"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/principal"
)

type purgeControlledSpace struct {
	spacev1.SpaceServiceClient
	call func(context.Context, *spacev1.GetBotPurgeHandoffRequest) (*spacev1.GetBotPurgeHandoffResponse, error)
}

func (c purgeControlledSpace) GetBotPurgeHandoff(ctx context.Context, q *spacev1.GetBotPurgeHandoffRequest, _ ...grpc.CallOption) (*spacev1.GetBotPurgeHandoffResponse, error) {
	return c.call(ctx, q)
}

type purgeControlledMessaging struct {
	messagingv1.MessagingServiceClient
	call func(context.Context, *messagingv1.GetBotPurgeChatCoverageRequest) (*messagingv1.GetBotPurgeChatCoverageResponse, error)
}

func (c purgeControlledMessaging) GetBotPurgeChatCoverage(ctx context.Context, q *messagingv1.GetBotPurgeChatCoverageRequest, _ ...grpc.CallOption) (*messagingv1.GetBotPurgeChatCoverageResponse, error) {
	return c.call(ctx, q)
}

func controlledPurgeOriginals(t *testing.T, p *commonv1.SpacePurgeRequest) (*spacev1.BotPurgeRoleHandoff, *spacev1.BotPurgeMessagingHandoff) {
	t.Helper()
	marshal := func(m proto.Message) []byte {
		b, e := (proto.MarshalOptions{Deterministic: true}).Marshal(m)
		require.NoError(t, e)
		return b
	}
	hash := func(domain string, b []byte) []byte {
		h := sha256.Sum256(append([]byte(domain+"\x00"), b...))
		return append([]byte(nil), h[:]...)
	}
	roleq := &rolev1.RetireSpaceRequest{ProtocolVersion: 1, SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, Manifest: proto.Clone(p.Manifest).(*commonv1.ManifestBinding), PurgeDecidedAt: proto.Clone(p.PurgeDecidedAt).(*timestamppb.Timestamp)}
	rb := marshal(roleq)
	rh := hash("voice.role.v1.RetireSpaceRequest", rb)
	rr := &rolev1.RetireSpaceReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, State: rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_RETIRED, RequestSha256: rh, ManifestSha256: append([]byte(nil), p.Manifest.ManifestSha256...), RetiredAt: timestamppb.Now()}
	rrb := marshal(rr)
	role := &spacev1.BotPurgeRoleHandoff{OriginalReceipt: rr, OriginalRequestBytes: rb, OriginalRequestSha256: rh, OriginalReceiptBytes: rrb, OriginalReceiptSha256: hash("voice.role.v1.RetireSpaceReceipt", rrb)}
	mp := proto.Clone(p).(*commonv1.SpacePurgeRequest)
	mp.ParticipantId = commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING
	mb := marshal(&messagingv1.PurgeSpaceRequest{Purge: mp})
	mh := hash("voice.messaging.v1.PurgeSpaceRequest", mb)
	mr := &commonv1.SpacePurgeReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), SpaceId: p.SpaceId, DeletionOperationId: p.DeletionOperationId, Generation: p.Generation, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING, State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: mh, CompletedAt: timestamppb.Now()}
	mrb := marshal(mr)
	return role, &spacev1.BotPurgeMessagingHandoff{OriginalReceipt: mr, OriginalRequestBytes: mb, OriginalRequestSha256: mh, OriginalReceiptBytes: mrb, OriginalReceiptSha256: hash("voice.common.v1.SpacePurgeReceipt", mrb)}
}

func TestPurgeReadWorkerUsesExactPurePrincipalAndRetainsPendingNativeWork(t *testing.T) {
	if testing.Short() {
		t.Skip("real Bot PostgreSQL c89 worker; controlled RSA receivers")
	}
	for _, chatCount := range []int{0, 1, 1001} {
		t.Run(strconv.Itoa(chatCount)+" exact eligible chats", func(t *testing.T) {
			hasChat := chatCount > 0
			ctx := t.Context()
			pool := integrationtest.StartPostgres(t, ctx, "bot_purge_reader", "")
			_, err := pool.Exec(ctx, migrationSQL(t))
			require.NoError(t, err)
			st := &store.BotStore{Pool: pool}
			actor, space := uuid.New(), uuid.New()
			bot, _, err := st.CreateBot(ctx, uuid.New(), "P3 reader", "", `[]`, actor)
			require.NoError(t, err)
			var chats []uuid.UUID
			for i := 0; i < chatCount; i++ {
				chats = append(chats, uuid.New())
			}
			sort.Slice(chats, func(i, j int) bool { return chats[i].String() < chats[j].String() })
			_, err = st.PrepareInstallation(ctx, bot.ID, space, actor, chats)
			require.NoError(t, err)
			manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32), ItemCount: uint64(len(chats))}
			f := &commonv1.SpaceLifecycleFenceRequest{ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: uuid.NewString(), Generation: 1, DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest}
			_, err = st.ApplySpaceLifecycleFence(ctx, f)
			require.NoError(t, err)
			f.Generation = 2
			f.DesiredState = commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
			_, err = st.ApplySpaceLifecycleFence(ctx, f)
			require.NoError(t, err)
			p := &commonv1.SpacePurgeRequest{ProtocolVersion: 1, SpaceId: f.SpaceId, DeletionOperationId: f.DeletionOperationId, Generation: 2, ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_BOT, Manifest: manifest, PurgeDecidedAt: timestamppb.Now()}
			_, err = st.PurgeSpace(ctx, p)
			require.ErrorIs(t, err, store.ErrDeletionPending)
			issuer, key := installationTestSigningKey(t)
			seen := map[string]bool{}
			verify := func(call context.Context, aud, method, id string, q proto.Message) {
				md, ok := metadata.FromOutgoingContext(call)
				require.True(t, ok)
				require.Equal(t, []string{id}, md.Get("x-request-id"))
				require.Empty(t, md.Get("x-voice-profile-id"))
				require.Empty(t, md.Get("x-voice-account-id"))
				h, e := principal.RequestHash(q)
				require.NoError(t, e)
				proof, e := principal.VerifyService(call, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: aud, ExpectedRPC: method, ExpectedRequestID: id, ExpectedRequestHash: h, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return key, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
					if seen[jti] {
						return errors.New("replay")
					}
					seen[jti] = true
					return nil
				}})
				require.NoError(t, e)
				require.Equal(t, "service:bot", proof.Subject)
				require.Empty(t, proof.AccountID)
				require.Empty(t, proof.ProfileID)
				require.Zero(t, proof.SessionEpoch)
			}
			role, msg := controlledPurgeOriginals(t, p)
			spaceCalls, msgCalls, pass := 0, 0, 0
			secondBatchFailed := false
			svc := &grpcsvc.BotGRPC{Store: st, PrincipalIssuer: issuer}
			svc.InstallationSpace = purgeControlledSpace{call: func(call context.Context, q *spacev1.GetBotPurgeHandoffRequest) (*spacev1.GetBotPurgeHandoffResponse, error) {
				spaceCalls++
				verify(call, "space", spacev1.SpaceService_GetBotPurgeHandoff_FullMethodName, q.RequestId, q)
				require.Equal(t, p.SpaceId, q.SpaceId)
				require.Equal(t, p.DeletionOperationId, q.DeletionOperationId)
				require.True(t, proto.Equal(p.Manifest, q.Manifest))
				if pass == 0 {
					return nil, status.Error(codes.Unavailable, "controlled accepted-ledger dependency outage")
				}
				r := &spacev1.GetBotPurgeHandoffResponse{ProtocolVersion: 1, RequestId: q.RequestId, SpaceId: q.SpaceId, DeletionOperationId: q.DeletionOperationId, Generation: q.Generation, Manifest: proto.Clone(q.Manifest).(*commonv1.ManifestBinding), Role: proto.Clone(role).(*spacev1.BotPurgeRoleHandoff)}
				if hasChat {
					r.Messaging = proto.Clone(msg).(*spacev1.BotPurgeMessagingHandoff)
				}
				if pass == 1 {
					r.Role = nil
				}
				return r, nil
			}}
			if hasChat {
				svc.CleanupMessaging = purgeControlledMessaging{call: func(call context.Context, q *messagingv1.GetBotPurgeChatCoverageRequest) (*messagingv1.GetBotPurgeChatCoverageResponse, error) {
					msgCalls++
					verify(call, "messaging", messagingv1.MessagingService_GetBotPurgeChatCoverage_FullMethodName, q.RequestId, q)
					require.NotEmpty(t, q.ChatIds)
					require.LessOrEqual(t, len(q.ChatIds), 1000)
					want := []string{}
					if q.ChatIds[0] == chats[0].String() {
						for i := 0; i < len(chats) && i < 1000; i++ {
							want = append(want, chats[i].String())
						}
					} else {
						require.Equal(t, 1001, len(chats))
						want = []string{chats[1000].String()}
					}
					require.Equal(t, want, q.ChatIds)
					if chatCount == 1001 && len(q.ChatIds) == 1 && !secondBatchFailed {
						secondBatchFailed = true
						return nil, status.Error(codes.Unavailable, "controlled second read batch unavailable after prefix was saved")
					}
					h, e := principal.RequestHash(q)
					require.NoError(t, e)
					digest, e := hex.DecodeString(strings.TrimPrefix(h, "sha256:"))
					require.NoError(t, e)
					var coverage []*messagingv1.BotPurgeChatCoverage
					for _, id := range q.ChatIds {
						coverage = append(coverage, &messagingv1.BotPurgeChatCoverage{ChatId: id, Covered: pass != 2})
					}
					return &messagingv1.GetBotPurgeChatCoverageResponse{ProtocolVersion: 1, RequestId: q.RequestId, SpaceId: q.SpaceId, DeletionOperationId: q.DeletionOperationId, Generation: q.Generation, Manifest: proto.Clone(q.Manifest).(*commonv1.ManifestBinding), SourceScheduleGeneration: 7, RequestSha256: digest, Coverage: coverage}, nil
				}}
			}
			for pass = 0; pass < 6; pass++ {
				_, err = pool.Exec(ctx, `UPDATE bot_space_purge_handoffs SET next_attempt_at=clock_timestamp() WHERE space_id=$1`, space)
				require.NoError(t, err)
				worked, e := svc.ProcessNextBotPurgeHandoff(ctx)
				require.True(t, worked)
				if pass == 0 || (chatCount == 1001 && pass == 3) {
					require.Equal(t, codes.Unavailable, status.Code(e))
				} else {
					require.ErrorIs(t, e, store.ErrDeletionPending)
				}
				var saved int
				require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_sources WHERE space_id=$1 AND coverage_bytes IS NOT NULL`, space).Scan(&saved))
				threshold := 2
				if hasChat {
					threshold = 3
				}
				if pass < threshold {
					require.Zero(t, saved)
				} else {
					require.Equal(t, 1, saved)
				}
				if chatCount == 1001 && pass >= 3 {
					var raw []byte
					require.NoError(t, pool.QueryRow(ctx, `SELECT coverage_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&raw))
					var envelope struct {
						Proofs []json.RawMessage `json:"proofs"`
					}
					require.NoError(t, json.Unmarshal(raw, &envelope))
					if pass == 3 {
						require.Len(t, envelope.Proofs, 1, "first thousand are durable before second batch retry")
					} else {
						require.Len(t, envelope.Proofs, 2, "resume must append only the missing final chat")
					}
				}
			}
			if hasChat {
				require.Equal(t, 4, spaceCalls)
				if chatCount == 1001 {
					require.Equal(t, 4, msgCalls)
					require.True(t, secondBatchFailed)
				} else {
					require.Equal(t, 2, msgCalls)
				}
			} else {
				require.Equal(t, 3, spaceCalls)
				require.Zero(t, msgCalls)
			}
			if chatCount == 1 {
				var raw []byte
				require.NoError(t, pool.QueryRow(ctx, `SELECT coverage_bytes FROM bot_space_purge_sources WHERE space_id=$1 AND bot_id=$2`, space, bot.ID).Scan(&raw))
				var envelope struct {
					Kind         string                                                                       `json:"kind"`
					Version      int                                                                          `json:"version"`
					SourceSHA256 []byte                                                                       `json:"source_sha256"`
					Proofs       []struct{ Purge, HandoffRequest, Handoff, CoverageRequest, Coverage []byte } `json:"proofs"`
				}
				require.NoError(t, json.Unmarshal(raw, &envelope))
				var bad spacev1.GetBotPurgeHandoffResponse
				require.NoError(t, proto.Unmarshal(envelope.Proofs[0].Handoff, &bad))
				bad.Role.OriginalReceipt.State = rolev1.RoleRetirementState_ROLE_RETIREMENT_STATE_UNSPECIFIED
				envelope.Proofs[0].Handoff, err = (proto.MarshalOptions{Deterministic: true}).Marshal(&bad)
				require.NoError(t, err)
				raw, err = json.Marshal(envelope)
				require.NoError(t, err)
				digest := sha256.Sum256(raw)
				_, err = pool.Exec(ctx, `UPDATE bot_space_purge_sources SET coverage_bytes=$3,coverage_sha256=$4 WHERE space_id=$1 AND bot_id=$2`, space, bot.ID, raw, digest[:])
				require.NoError(t, err)
				_, err = pool.Exec(ctx, `UPDATE bot_space_purge_handoffs SET next_attempt_at=clock_timestamp() WHERE space_id=$1`, space)
				require.NoError(t, err)
				beforeSpace, beforeMsg := spaceCalls, msgCalls
				worked, e := svc.ProcessNextBotPurgeHandoff(ctx)
				require.True(t, worked)
				require.ErrorIs(t, e, store.ErrSpaceLifecycleConflict)
				require.Equal(t, beforeSpace, spaceCalls)
				require.Equal(t, beforeMsg, msgCalls, "digest-consistent invalid typed proof cannot be trusted or rehydrated")
			}
			var sourcesPending, purgeReceipts, roots int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_purge_sources WHERE space_id=$1 AND state='PENDING' AND covered_at IS NULL`, space).Scan(&sourcesPending))
			require.Equal(t, 1, sourcesPending)
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_space_lifecycle_purge_receipts WHERE space_id=$1`, space).Scan(&purgeReceipts))
			require.Zero(t, purgeReceipts)
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM bot_deletion_operations WHERE bot_id=$1`, bot.ID).Scan(&roots))
			require.Zero(t, roots)
		})
	}
}
