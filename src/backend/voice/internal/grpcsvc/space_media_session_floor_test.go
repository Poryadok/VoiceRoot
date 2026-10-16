package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	authv1 "voice.app/voice/auth/v1"
	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/voice/internal/livekit"
	"voice/backend/voice/internal/sessionfloor"
	voicestore "voice/backend/voice/internal/store"
)

func TestSpaceMediaAuthFloorDenialHasNoEffects(t *testing.T) {
	auth := newSpaceFloorAuthRPC(t)
	for _, path := range []string{"new join", "authenticated rejoin", "token issue", "token reissue"} {
		t.Run(path, func(t *testing.T) {
			for _, tc := range []struct {
				name  string
				floor int64
				err   error
				code  codes.Code
			}{
				{name: "stale epoch", floor: 10, code: codes.Unauthenticated},
				{name: "Auth unavailable", err: status.Error(codes.Unavailable, "authority unavailable"), code: codes.Unavailable},
				{name: "missing floor", code: codes.Unavailable},
				{name: "invalid floor", floor: -1, code: codes.Unavailable},
				{name: "missing account", err: status.Error(codes.NotFound, "account unavailable"), code: codes.Unauthenticated},
			} {
				t.Run(tc.name, func(t *testing.T) {
					auth.set(9, nil)
					f := newSpaceFloorVoiceFixture(auth.client)
					if path != "new join" {
						joined, err := f.join(t)
						require.NoError(t, err)
						require.NotEmpty(t, joined.GetVoiceSession().GetRoomId())
					}
					if path == "token reissue" {
						issued, err := f.token(t)
						require.NoError(t, err)
						require.NotEmpty(t, issued.GetJwt())
					}
					// The same real client now reads changed authority. No value from
					// successful admission/issuance may authorize the next request.
					auth.set(tc.floor, tc.err)
					before := f.snapshot(t)
					var err error
					if path == "new join" || path == "authenticated rejoin" {
						var response *callsv1.JoinVoiceRoomResponse
						response, err = f.join(t)
						require.Nil(t, response)
					} else {
						var response *callsv1.GetJoinTokenResponse
						response, err = f.token(t)
						require.Nil(t, response, "denied authority must not return a media JWT")
					}
					require.Equal(t, tc.code, status.Code(err))
					require.Equal(t, before, f.snapshot(t), "denial must not change sessions, roster, floors, admission journal or events")
					auth.requireReads(t, f.accountID, 1)
				})
			}
		})
	}
}

func TestSpaceMediaCurrentAuthFloorPermitsIssueReissueOnlyWithSpaceAndRoleAuthority(t *testing.T) {
	auth := newSpaceFloorAuthRPC(t)
	auth.set(9, nil)
	f := newSpaceFloorVoiceFixture(auth.client)
	joined, err := f.join(t)
	require.NoError(t, err)
	call, err := f.calls.GetCall(t.Context(), joined.GetVoiceSession().GetRoomId())
	require.NoError(t, err)
	require.Equal(t, f.accountID, call.SpaceMedia[f.profileID].AccountID)
	require.Equal(t, uint64(9), call.SpaceMedia[f.profileID].Issued.SessionEpoch)
	require.Len(t, f.prepared, 1)
	require.Equal(t, []string{"voice.call_started", "voice.member_joined"}, preparedSpaceMediaEventSubjects(t, f.prepared))
	auth.requireReads(t, f.accountID, 2)
	for attempt := 0; attempt < 2; attempt++ {
		auth.set(9, nil)
		issued, err := f.token(t)
		require.NoError(t, err)
		require.NotEmpty(t, issued.GetJwt())
		require.Equal(t, "ws://livekit.test", issued.GetLivekitUrl())
		require.Equal(t, time.Minute, issued.GetExpiresAt().AsTime().Sub(f.svc.now()))
		auth.requireReads(t, f.accountID, 2)
	}
	auth.set(9, nil)
	rejoined, err := f.join(t)
	require.NoError(t, err)
	require.Equal(t, call.RoomID, rejoined.GetVoiceSession().GetRoomId())
	current, err := f.calls.GetCall(t.Context(), call.RoomID)
	require.NoError(t, err)
	require.Equal(t, call, current, "authenticated rejoin preserves the original session and media incarnation")
	require.Len(t, f.prepared, 1, "authenticated rejoin must reuse the confirmed admission")
	auth.requireReads(t, f.accountID, 1)
	// A current Auth floor does not replace canonical membership or Role policy.
	for _, authority := range []string{"Space membership", "Role grant"} {
		t.Run(authority, func(t *testing.T) {
			auth.set(9, nil)
			f.access.result.Member = authority != "Space membership"
			f.svc.SpaceVoiceRoomGrants = staticSpaceMediaGrants{grants: CanonicalVoiceRoomGrants{
				PolicyEpoch: 7, CanJoin: authority != "Role grant", CanSubscribe: true,
			}}
			before := f.snapshot(t)
			response, err := f.token(t)
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.Nil(t, response)
			require.Equal(t, before, f.snapshot(t))
		})
	}
}

// The handler receives a real signed delegated-user context through the existing
// strict helper. Its independent Auth authority checker is the production gRPC
// client below, rather than the healthy ingress-only fixture checker.
type spaceFloorVoiceFixture struct {
	svc                                        *VoiceGRPC
	calls                                      *spaceFloorCallStore
	admissions                                 *spaceFloorAdmissionRecorder
	access                                     *canonicalAccessResolver
	events                                     *recordingEvents
	prepared                                   []voicestore.SpaceMediaAdmission
	spaceID, voiceRoomID, accountID, profileID string
}

func newSpaceFloorVoiceFixture(client *sessionfloor.Client) *spaceFloorVoiceFixture {
	f := &spaceFloorVoiceFixture{
		spaceID: uuid.NewString(), voiceRoomID: uuid.NewString(), accountID: uuid.NewString(), profileID: uuid.NewString(),
		events: &recordingEvents{},
	}
	f.svc = newTestVoiceService(time.Unix(1700000000, 0).UTC(), f.events)
	f.calls = &spaceFloorCallStore{CallStore: f.svc.Calls}
	f.admissions = &spaceFloorAdmissionRecorder{readySpaceMediaAdmission: readySpaceMediaAdmission{preparedAdmissions: &f.prepared}}
	f.access = &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: f.spaceID, Active: true, Member: true, AccessEpoch: 11}}
	f.svc.Calls, f.svc.SpaceMediaAdmissions = f.calls, f.admissions
	f.svc.SpaceMediaReady = true
	f.svc.VoiceRoomAccessResolver = f.access
	f.svc.Roles = &canonicalRolePermissions{}
	f.svc.SpaceVoiceRoomGrants = staticSpaceMediaGrants{grants: CanonicalVoiceRoomGrants{PolicyEpoch: 7, CanJoin: true, CanSubscribe: true}}
	f.svc.SpaceTokens = livekit.NewSpaceTokenIssuer("test-key", "test-secret", "ws://livekit.test", time.Minute)
	f.svc.SessionEpochChecker = client
	return f
}

func (f *spaceFloorVoiceFixture) join(t *testing.T) (*callsv1.JoinVoiceRoomResponse, error) {
	t.Helper()
	request := &callsv1.JoinVoiceRoomRequest{VoiceRoomId: f.voiceRoomID, Space: &spacev1.SpaceRef{Id: f.spaceID}}
	ctx := verifiedVoiceUserContext(t, callsv1.VoiceService_JoinVoiceRoom_FullMethodName, request, f.accountID, f.profileID, 9)
	return f.svc.JoinVoiceRoom(ctx, request)
}

func (f *spaceFloorVoiceFixture) token(t *testing.T) (*callsv1.GetJoinTokenResponse, error) {
	t.Helper()
	call, err := f.calls.GetCallByVoiceRoomID(t.Context(), f.voiceRoomID)
	require.NoError(t, err)
	request := &callsv1.GetJoinTokenRequest{RoomId: call.RoomID}
	ctx := verifiedVoiceUserContext(t, callsv1.VoiceService_GetJoinToken_FullMethodName, request, f.accountID, f.profileID, 9)
	return f.svc.GetJoinToken(ctx, request)
}

func (f *spaceFloorVoiceFixture) snapshot(t *testing.T) string {
	t.Helper()
	calls, err := f.calls.ListActiveSpaceVoiceCalls(t.Context(), f.spaceID)
	require.NoError(t, err)
	active, err := f.calls.GetActiveCall(t.Context(), f.profileID)
	require.True(t, err == nil || errors.Is(err, voicestore.ErrNotFound))
	floors, err := f.calls.GetSpaceMediaEpochFloors(t.Context(), f.spaceID)
	require.NoError(t, err)
	progress, err := f.calls.GetSpaceMediaEpochProgress(t.Context(), f.spaceID)
	require.NoError(t, err)
	encoded, err := json.Marshal(struct {
		Calls          []voicestore.Call
		Active         voicestore.Call
		Floors         voicestore.SpaceMediaEpochFloors
		Progress       voicestore.SpaceMediaEpochProgress
		StoreWrites    []string
		AdmissionCalls []string
		Prepared       []voicestore.SpaceMediaAdmission
		EventCounts    []int
	}{calls, active, floors, progress, f.calls.writes, f.admissions.calls, f.prepared, []int{
		len(f.events.incoming), len(f.events.accepted), len(f.events.declined), len(f.events.missed), len(f.events.ended),
		len(f.events.states), len(f.events.started), len(f.events.stopped), len(f.events.startedCall), len(f.events.memberJoined),
	}})
	require.NoError(t, err)
	return string(encoded)
}

type spaceFloorCallStore struct {
	voicestore.CallStore
	writes []string
}

func (s *spaceFloorCallStore) CreateCall(ctx context.Context, call voicestore.Call) (voicestore.Call, error) {
	s.writes = append(s.writes, "CreateCall")
	return s.CallStore.CreateCall(ctx, call)
}

func (s *spaceFloorCallStore) AdmitSpaceMediaParticipant(ctx context.Context, room string, participant voicestore.SpaceMediaParticipant, max int) (voicestore.Call, error) {
	s.writes = append(s.writes, "AdmitSpaceMediaParticipant")
	return s.CallStore.AdmitSpaceMediaParticipant(ctx, room, participant, max)
}

func (s *spaceFloorCallStore) RaiseSpaceMediaEpochFloor(ctx context.Context, space string, kind voicestore.SpaceMediaEpochKind, epoch uint64) (voicestore.SpaceMediaEpochFloors, error) {
	s.writes = append(s.writes, "RaiseSpaceMediaEpochFloor")
	return s.CallStore.RaiseSpaceMediaEpochFloor(ctx, space, kind, epoch)
}

type spaceFloorAdmissionRecorder struct {
	readySpaceMediaAdmission
	calls []string
}

func (r *spaceFloorAdmissionRecorder) ClaimRoomHead(ctx context.Context, voiceRoom, space, room string, operation uuid.UUID, create bool) (voicestore.SpaceMediaRoomHead, error) {
	r.calls = append(r.calls, "ClaimRoomHead")
	return r.readySpaceMediaAdmission.ClaimRoomHead(ctx, voiceRoom, space, room, operation, create)
}

func (r *spaceFloorAdmissionRecorder) Prepare(ctx context.Context, admission voicestore.SpaceMediaAdmission) error {
	r.calls = append(r.calls, "Prepare")
	return r.readySpaceMediaAdmission.Prepare(ctx, admission)
}

func (r *spaceFloorAdmissionRecorder) Fence(ctx context.Context, admission voicestore.SpaceMediaAdmission) error {
	r.calls = append(r.calls, "Fence")
	return r.readySpaceMediaAdmission.Fence(ctx, admission)
}

func (r *spaceFloorAdmissionRecorder) MarkCommitted(ctx context.Context, operation uuid.UUID, generation string) error {
	r.calls = append(r.calls, "MarkCommitted")
	return r.readySpaceMediaAdmission.MarkCommitted(ctx, operation, generation)
}

func (r *spaceFloorAdmissionRecorder) MarkProjectionApplied(ctx context.Context, operation uuid.UUID, generation string) error {
	r.calls = append(r.calls, "MarkProjectionApplied")
	return r.readySpaceMediaAdmission.MarkProjectionApplied(ctx, operation, generation)
}

func (r *spaceFloorAdmissionRecorder) ConfirmProjection(ctx context.Context, operation uuid.UUID, generation string) error {
	r.calls = append(r.calls, "ConfirmProjection")
	return r.readySpaceMediaAdmission.ConfirmProjection(ctx, operation, generation)
}

func (r *spaceFloorAdmissionRecorder) ConfirmRoomHeadOpen(ctx context.Context, admission voicestore.SpaceMediaAdmission) error {
	r.calls = append(r.calls, "ConfirmRoomHeadOpen")
	return r.readySpaceMediaAdmission.ConfirmRoomHeadOpen(ctx, admission)
}

type spaceFloorAuthRead struct {
	accountID string
	requestID string
}

type spaceFloorAuthRPC struct {
	authv1.UnimplementedAuthServiceServer
	mu       sync.Mutex
	key      *rsa.PublicKey
	client   *sessionfloor.Client
	floor    int64
	err      error
	reads    []spaceFloorAuthRead
	failures []error
	replays  map[string]bool
}

func newSpaceFloorAuthRPC(t *testing.T) *spaceFloorAuthRPC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "voice", KeyID: "space-floor-test", PrivateKey: key})
	require.NoError(t, err)
	auth := &spaceFloorAuthRPC{key: &key.PublicKey, replays: map[string]bool{}}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	authv1.RegisterAuthServiceServer(server, auth)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///space-floor-auth", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	auth.client = sessionfloor.New(authv1.NewAuthServiceClient(conn), issuer)
	return auth
}

func (a *spaceFloorAuthRPC) set(floor int64, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.floor, a.err, a.reads, a.failures = floor, err, nil, nil
}

func (a *spaceFloorAuthRPC) requireReads(t *testing.T, account string, count int) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	require.Empty(t, a.failures, "an invalid service principal must not masquerade as an authority failure")
	require.Len(t, a.reads, count, "each denial makes one fresh bounded lookup, with no retry/cache fallback")
	for _, read := range a.reads {
		require.Equal(t, account, read.accountID, "Auth account comes from the verified delegated-user context")
		require.NotEmpty(t, read.requestID)
	}
}

func (a *spaceFloorAuthRPC) GetVoiceSessionEpochFloor(ctx context.Context, request *authv1.GetVoiceSessionEpochFloorRequest) (*authv1.GetVoiceSessionEpochFloorResponse, error) {
	transport, err := principal.IncomingMetadata(ctx)
	if err == nil {
		requestID, parseErr := uuid.Parse(transport.RequestID)
		if parseErr != nil || requestID == uuid.Nil || requestID.String() != transport.RequestID {
			err = errors.New("invalid Auth lookup request ID")
		}
	}
	if err == nil {
		hash, hashErr := principal.RequestHash(request)
		err = hashErr
		if err == nil {
			var verified principal.Principal
			verified, err = principal.VerifyService(ctx, transport.BearerToken, principal.VerifyConfig{
				ExpectedIssuer: "voice", ExpectedAudience: sessionfloor.Audience, ExpectedRPC: sessionfloor.RPC,
				ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash,
				KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
					if issuer != "voice" || kid != "space-floor-test" {
						return nil, errors.New("unexpected Auth lookup signer")
					}
					return a.key, nil
				},
				ReplayGuard: func(_ context.Context, issuer, jti string, _ time.Time) error {
					a.mu.Lock()
					defer a.mu.Unlock()
					key := issuer + ":" + jti
					if a.replays[key] {
						return errors.New("replayed Auth lookup")
					}
					a.replays[key] = true
					return nil
				},
			})
			if err == nil && verified.Subject != "service:voice" {
				err = errors.New("unexpected Auth lookup subject")
			}
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.failures = append(a.failures, err)
		return nil, status.Error(codes.Unauthenticated, "invalid Voice service principal")
	}
	deadline, bounded := ctx.Deadline()
	if !bounded || time.Until(deadline) <= 0 || time.Until(deadline) > 2*time.Second {
		a.failures = append(a.failures, errors.New("unbounded Auth lookup"))
		return nil, status.Error(codes.Unavailable, "unbounded Auth lookup")
	}
	a.reads = append(a.reads, spaceFloorAuthRead{accountID: request.GetAccountId(), requestID: transport.RequestID})
	if a.err != nil {
		return nil, a.err
	}
	return &authv1.GetVoiceSessionEpochFloorResponse{SessionEpochFloor: a.floor}, nil
}
