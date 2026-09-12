package roomlifecycle

import (
	"context"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"voice/backend/pkg/integrationtest"
)

// Shared immutable fixtures for the independently approved R22.3 D2/D3 RED
// contract. Production-facing tests are build-tagged in the companion file so
// ordinary tests can run harness checks before the expected RED compile.

type r223ReceiptVector struct {
	name      string
	method    string
	receipt   LifecycleReceipt
	wantBytes string
	wantHash  string
}

var r223FixedTime = time.Date(2033, 11, 14, 22, 13, 20, 123000000, time.UTC)

func r223Digest(seed byte) LifecycleDigest {
	var digest LifecycleDigest
	for i := range digest {
		digest[i] = seed + byte(i)
	}
	return digest
}

//nolint:unused // shared with the build-tagged production contract test
func r223MethodName(method LifecycleMethod) string {
	switch method {
	case LifecycleMethodJoin:
		return "join"
	case LifecycleMethodLeave:
		return "leave"
	case LifecycleMethodSelfMove:
		return "self_move"
	case LifecycleMethodModeratorMove:
		return "moderator_move"
	default:
		panic("invalid test method")
	}
}

func r223ReceiptVectors(t *testing.T) []r223ReceiptVector {
	t.Helper()
	id := func(value string) uuid.UUID { return uuid.MustParse(value) }
	op := id("11111111-1111-4111-8111-111111111111")
	actor := id("22222222-2222-4222-8222-222222222222")
	subject := id("33333333-3333-4333-8333-333333333333")
	space := id("44444444-4444-4444-8444-444444444444")
	source := id("55555555-5555-4555-8555-555555555555")
	destination := id("66666666-6666-4666-8666-666666666666")
	room := id("77777777-7777-4777-8777-777777777777")
	media := id("88888888-8888-4888-8888-888888888888")
	sourceRoster, destinationRoster := int64(7), int64(8)
	spaceEpoch, roleEpoch := int64(9), int64(10)
	authorization := r223Digest(0)
	base := LifecycleReceipt{OperationID: op, ActorProfileID: actor, SubjectProfileID: subject, SpaceID: space}

	joined := base
	joined.Method, joined.Outcome = LifecycleMethodJoin, LifecycleOutcomeJoined
	joined.DestinationVoiceRoomID, joined.RoomID = &destination, &room
	joined.DestinationRosterVersion, joined.MediaEpoch = &destinationRoster, &media
	joined.SpaceAccessEpoch, joined.RolePolicyEpoch, joined.AuthorizationDigest = &spaceEpoch, &roleEpoch, &authorization
	joinNoOp := joined
	joinNoOp.Outcome = LifecycleOutcomeNoOp
	left := base
	left.Method, left.Outcome = LifecycleMethodLeave, LifecycleOutcomeLeft
	left.SourceVoiceRoomID, left.RoomID, left.SourceRosterVersion = &source, &room, &sourceRoster
	leaveNoOp := base
	leaveNoOp.Method, leaveNoOp.Outcome = LifecycleMethodLeave, LifecycleOutcomeNoOp
	selfMoved := base
	selfMoved.Method, selfMoved.Outcome = LifecycleMethodSelfMove, LifecycleOutcomeMoved
	selfMoved.SourceVoiceRoomID, selfMoved.DestinationVoiceRoomID, selfMoved.RoomID = &source, &destination, &room
	selfMoved.SourceRosterVersion, selfMoved.DestinationRosterVersion, selfMoved.MediaEpoch = &sourceRoster, &destinationRoster, &media
	selfMoved.SpaceAccessEpoch, selfMoved.RolePolicyEpoch, selfMoved.AuthorizationDigest = &spaceEpoch, &roleEpoch, &authorization
	moderatorMoved := selfMoved
	moderatorMoved.Method = LifecycleMethodModeratorMove

	vectors := []r223ReceiptVector{
		{"join_joined", "join", joined, "0a2431313131313131312d313131312d343131312d383131312d313131313131313131313131122432323232323232322d323232322d343232322d383232322d3232323232323232323232321a2433333333333333332d333333332d343333332d383333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d383434342d34343434343434343434343428013001422436363636363636362d363636362d343636362d383636362d3636363636363636363636364a2437373737373737372d373737372d343737372d383737372d3737373737373737373737375808622438383838383838382d383838382d343838382d383838382d3838383838383838383838386809700a7a20000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f", "acbd00d278598d613a11d9be8d3951e93164d6f87f30e1a8233f6a6b003e95e1"},
		{"join_no_op", "join", joinNoOp, "0a2431313131313131312d313131312d343131312d383131312d313131313131313131313131122432323232323232322d323232322d343232322d383232322d3232323232323232323232321a2433333333333333332d333333332d343333332d383333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d383434342d34343434343434343434343428013004422436363636363636362d363636362d343636362d383636362d3636363636363636363636364a2437373737373737372d373737372d343737372d383737372d3737373737373737373737375808622438383838383838382d383838382d343838382d383838382d3838383838383838383838386809700a7a20000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f", "8ec27c432bb3d902d4aaff9ed7c7053bbb54106b7df2e2a4d0dd4f9033c14c34"},
		{"leave_left", "leave", left, "0a2431313131313131312d313131312d343131312d383131312d313131313131313131313131122432323232323232322d323232322d343232322d383232322d3232323232323232323232321a2433333333333333332d333333332d343333332d383333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d383434342d343434343434343434343434280230023a2435353535353535352d353535352d343535352d383535352d3535353535353535353535354a2437373737373737372d373737372d343737372d383737372d3737373737373737373737375007", "53485a5a19d848b0f7fe6cdb69f5ff6e313b3ef8bbd0058014d4577032476873"},
		{"leave_no_op", "leave", leaveNoOp, "0a2431313131313131312d313131312d343131312d383131312d313131313131313131313131122432323232323232322d323232322d343232322d383232322d3232323232323232323232321a2433333333333333332d333333332d343333332d383333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d383434342d34343434343434343434343428023004", "33d308bbc349ebaf68b77d9d886903e6dffa484102e1b0351a3fdabef3a1ace2"},
		{"self_move", "self_move", selfMoved, "0a2431313131313131312d313131312d343131312d383131312d313131313131313131313131122432323232323232322d323232322d343232322d383232322d3232323232323232323232321a2433333333333333332d333333332d343333332d383333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d383434342d343434343434343434343434280330033a2435353535353535352d353535352d343535352d383535352d353535353535353535353535422436363636363636362d363636362d343636362d383636362d3636363636363636363636364a2437373737373737372d373737372d343737372d383737372d37373737373737373737373750075808622438383838383838382d383838382d343838382d383838382d3838383838383838383838386809700a7a20000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f", "45997cd28535ad31183b82524346a2e0d1edd93d1a06445922efe1f803f28519"},
		{"moderator_move", "moderator_move", moderatorMoved, "0a2431313131313131312d313131312d343131312d383131312d313131313131313131313131122432323232323232322d323232322d343232322d383232322d3232323232323232323232321a2433333333333333332d333333332d343333332d383333332d33333333333333333333333322260a2434343434343434342d343434342d343434342d383434342d343434343434343434343434280430033a2435353535353535352d353535352d343535352d383535352d353535353535353535353535422436363636363636362d363636362d343636362d383636362d3636363636363636363636364a2437373737373737372d373737372d343737372d383737372d37373737373737373737373750075808622438383838383838382d383838382d343838382d383838382d3838383838383838383838386809700a7a20000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f", "c9ffee72eb60ac24d50ed0f2ac19b16a0b98331ecc3c83ec905190b3888a627e"},
	}
	for _, vector := range vectors {
		encoded, hash, err := EncodeLifecycleReceipt(vector.receipt)
		require.NoError(t, err, "fixture %s must be a canonical R22.2 receipt", vector.name)
		require.Equal(t, vector.wantBytes, hex.EncodeToString(encoded), "fixture drift: update only after canonical receipt change")
		require.Equal(t, vector.wantHash, hex.EncodeToString(hash[:]), "fixture digest drift")
	}
	return vectors
}

func r223MiniredisLedger(t *testing.T) (*redis.Client, *RedisLedger) {
	t.Helper()
	mr := miniredis.RunT(t)
	mr.SetTime(r223FixedTime)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, NewRedisLedger(client)
}

func TestR223D2D3Harness_FrozenMiniredisClockDetectsAbsoluteExpiryMutation(t *testing.T) {
	client, _ := r223MiniredisLedger(t)
	ctx := context.Background()
	key := "r223:expiry-mutation-control"
	deadline := r223FixedTime.Add(time.Hour)
	require.NoError(t, client.HSet(ctx, key, "field", "value").Err())
	require.NoError(t, client.PExpireAt(ctx, key, deadline).Err())
	before, err := client.Do(ctx, "PEXPIRETIME", key).Int64()
	require.NoError(t, err)
	require.Equal(t, deadline.UnixMilli(), before)
	require.NoError(t, client.PExpireAt(ctx, key, deadline.Add(time.Millisecond)).Err())
	after, err := client.Do(ctx, "PEXPIRETIME", key).Int64()
	require.NoError(t, err)
	require.Equal(t, deadline.Add(time.Millisecond).UnixMilli(), after)
	require.NotEqual(t, before, after, "the frozen clock must still expose an absolute-expiry mutation")
}

func TestR223D2D3Harness_FrozenReceiptGoldensAndOriginKeysAreSelfConsistent(t *testing.T) {
	require.Len(t, r223ReceiptVectors(t), 6)
	key, err := redisMirrorKey(RedisMirrorKey{
		Origin:         "system_disconnect",
		ActorProfileID: uuid.MustParse("22222222-2222-4222-8222-222222222222"),
		OperationID:    uuid.MustParse("11111111-1111-4111-8111-111111111111"),
	})
	require.NoError(t, err)
	require.Equal(t,
		"voice:lifecycle:v2:op:system_disconnect:22222222-2222-4222-8222-222222222222:11111111-1111-4111-8111-111111111111",
		key,
	)
}

func TestR223D2D3Harness_MemoryLedgerHasNoRedisV2API(t *testing.T) {
	directory := filepath.Join(r22VoiceRepoRoot(t), "src", "backend", "voice", "internal", "roomlifecycle")
	paths, err := filepath.Glob(filepath.Join(directory, "*.go"))
	require.NoError(t, err)
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		require.NoError(t, parseErr)
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || len(function.Recv.List) != 1 {
				continue
			}
			receiver := function.Recv.List[0].Type
			if pointer, ok := receiver.(*ast.StarExpr); ok {
				receiver = pointer.X
			}
			identifier, ok := receiver.(*ast.Ident)
			if ok && identifier.Name == "MemoryLedger" {
				require.NotContains(t, []string{"InspectMirror", "AttachPending", "CompleteMirror"}, function.Name.Name, "MemoryLedger must remain outside durable Redis-v2")
			}
		}
	}
}

//nolint:unused // shared with the build-tagged production contract test
func cloneStringMap(source map[string]string) map[string]string {
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

const r223RedisImage = "redis:7.4-bookworm"

func r223StartRealRedis(t *testing.T, ctx context.Context) (string, *redis.Client) {
	t.Helper()
	if testing.Short() {
		t.Skip("R22.3 D2/D3 requires isolated real Redis 7")
	}
	request := testcontainers.ContainerRequest{Name: "voice-r223-d2d3-red-" + uuid.NewString(), Image: r223RedisImage, ExposedPorts: []string{"6379/tcp"}, WaitingFor: wait.ForLog("Ready to accept connections").WithStartupTimeout(30 * time.Second)}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: request, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	host, err := container.Host(ctx)
	require.NoError(t, err)
	var port nat.Port
	for attempt := 0; attempt < 50; attempt++ {
		port, err = container.MappedPort(ctx, "6379/tcp")
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, err)
	address := fmt.Sprintf("%s:%s", host, port.Port())
	client := redis.NewClient(&redis.Options{Addr: address, MaxRetries: 0})
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.Ping(ctx).Err())
	return address, client
}

func r223RequireRedis7(t *testing.T, ctx context.Context, client *redis.Client) {
	t.Helper()
	info, err := client.Info(ctx, "server").Result()
	require.NoError(t, err)
	require.Contains(t, info, "redis_version:7.")
	_, err = client.Time(ctx).Result()
	require.NoError(t, err)
	key := "voice:r223:d2d3:harness:" + uuid.NewString()
	deadline := time.Now().UTC().Add(10 * time.Second).Truncate(time.Millisecond)
	require.NoError(t, client.Set(ctx, key, "sentinel", 0).Err())
	require.NoError(t, client.PExpireAt(ctx, key, deadline).Err())
	got, err := client.Do(ctx, "PEXPIRETIME", key).Int64()
	require.NoError(t, err)
	require.Equal(t, deadline.UnixMilli(), got)
}

func TestR223D2D3Harness_RealPostgres16AndRedis7SupportNativeDeadlinePrimitives(t *testing.T) {
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "r223_d2d3_"+strings.ReplaceAll(uuid.NewString(), "-", ""), "")
	var versionText string
	require.NoError(t, pool.QueryRow(ctx, "SHOW server_version_num").Scan(&versionText))
	version, err := strconv.Atoi(versionText)
	require.NoError(t, err)
	require.GreaterOrEqual(t, version, 160000)
	_, client := r223StartRealRedis(t, ctx)
	r223RequireRedis7(t, ctx, client)
}
