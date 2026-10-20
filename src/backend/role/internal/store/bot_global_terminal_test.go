package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/principal"
)

func globalTerminalRequest() *rolev1.RegisterBotGlobalTerminalRequest {
	return &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: uuid.NewString(), DecisionId: uuid.NewString()}
}
func globalTerminalContext(t *testing.T, r *rolev1.RegisterBotGlobalTerminalRequest) context.Context {
	t.Helper()
	hash, err := principal.RequestHash(r)
	require.NoError(t, err)
	return principal.WithVerified(context.Background(), principal.Principal{Kind: "service", Issuer: "bot", Subject: "service:bot", Audience: "role", RPC: rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName, RequestID: r.DecisionId, RequestHash: hash})
}

func TestBotGlobalTerminalStore_ReplayRebindAndConcurrentDuplicate(t *testing.T) {
	ctx := context.Background()
	f := role26Fixture(t)
	app := role26Serving(t, f.Pool)
	s := &RoleStore{Pool: app}
	r := globalTerminalRequest()
	var wg sync.WaitGroup
	responses := make([]*rolev1.RegisterBotGlobalTerminalResponse, 2)
	errs := make([]error, 2)
	verified := globalTerminalContext(t, r)
	for i := range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); responses[i], errs[i] = s.RegisterBotGlobalTerminal(verified, r) }()
	}
	wg.Wait()
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.True(t, proto.Equal(responses[0], responses[1]))
	expected := responses[0]
	require.Equal(t, "role", expected.Receipt.Participant)
	require.Equal(t, rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED, expected.Receipt.Outcome)
	require.NotEqual(t, uuid.Nil, uuid.MustParse(expected.Receipt.ReceiptId))
	require.NoError(t, expected.Receipt.RegisteredAt.CheckValid())
	fresh, err := pgxpool.NewWithConfig(ctx, app.Config().Copy())
	require.NoError(t, err)
	defer fresh.Close()
	replay, err := (&RoleStore{Pool: fresh}).RegisterBotGlobalTerminal(globalTerminalContext(t, r), r)
	require.NoError(t, err)
	a, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(expected)
	b, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(replay)
	require.Equal(t, a, b)
	for _, change := range []func(*rolev1.RegisterBotGlobalTerminalRequest){func(x *rolev1.RegisterBotGlobalTerminalRequest) { x.BotId = uuid.NewString() }, func(x *rolev1.RegisterBotGlobalTerminalRequest) { x.DecisionId = uuid.NewString() }} {
		changed := proto.Clone(r).(*rolev1.RegisterBotGlobalTerminalRequest)
		change(changed)
		got, err := s.RegisterBotGlobalTerminal(globalTerminalContext(t, changed), changed)
		require.ErrorIs(t, err, ErrBotGlobalTerminalConflict)
		require.Nil(t, got)
	}
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_global_terminals").Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_actor_bindings").Scan(&count))
	require.Zero(t, count)
}

func TestBotGlobalTerminalStore_IndependentVerifiedBoundary(t *testing.T) {
	r := globalTerminalRequest()
	ctx := globalTerminalContext(t, r)
	p, _ := principal.FromContext(ctx)
	tests := []struct {
		name   string
		change func(*principal.Principal)
	}{
		{"kind", func(p *principal.Principal) { p.Kind = "bot_actor" }}, {"issuer", func(p *principal.Principal) { p.Issuer = "gateway" }},
		{"subject", func(p *principal.Principal) { p.Subject = "service:space" }}, {"audience", func(p *principal.Principal) { p.Audience = "space" }},
		{"method", func(p *principal.Principal) { p.RPC = rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName }}, {"request_id", func(p *principal.Principal) { p.RequestID = uuid.NewString() }},
		{"hash", func(p *principal.Principal) { p.RequestHash = "sha256:" + string(bytes.Repeat([]byte{'0'}, 64)) }}, {"account", func(p *principal.Principal) { p.AccountID = uuid.NewString() }},
		{"profile", func(p *principal.Principal) { p.ProfileID = uuid.NewString() }}, {"epoch", func(p *principal.Principal) { p.SessionEpoch = 1 }},
		{"actor", func(p *principal.Principal) { p.ActorProfileID = uuid.NewString() }}, {"space", func(p *principal.Principal) { p.SpaceID = uuid.NewString() }},
		{"installation", func(p *principal.Principal) { p.InstallationID = uuid.NewString() }}, {"scope", func(p *principal.Principal) { p.BotScope = "SPACE_MANAGE_ROLES" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bad := p
			tt.change(&bad)
			_, err := (&RoleStore{}).RegisterBotGlobalTerminal(principal.WithVerified(context.Background(), bad), r)
			require.ErrorIs(t, err, ErrBotGlobalTerminalDenied)
		})
	}
	_, err := (&RoleStore{}).RegisterBotGlobalTerminal(context.Background(), r)
	require.ErrorIs(t, err, ErrBotGlobalTerminalDenied)
	malformed := proto.Clone(r).(*rolev1.RegisterBotGlobalTerminalRequest)
	malformed.ProtoReflect().SetUnknown([]byte{0x20, 1})
	_, err = (&RoleStore{}).RegisterBotGlobalTerminal(globalTerminalContext(t, malformed), malformed)
	require.ErrorIs(t, err, ErrBotGlobalTerminalInvalid)
}

func TestBotGlobalTerminalStore_Known25BlockedOwnerProjection(t *testing.T) {
	ctx := context.Background()
	f := removalFixture(t) // historical helper now advances to25 only
	// The inherited helper is explicit source25, not a forced version marker.
	req := removalRequest()
	req.Purpose = "global_delete"
	owner := insertRemovalRole(t, f.Pool, req.SpaceId, uuid.NewString(), "Owner", true)
	_, err := f.Pool.Exec(ctx, "INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$2)", req.SpaceId, req.ActorProfileId, owner)
	require.NoError(t, err)
	got, err := (&RoleStore{Pool: f.Pool}).ApplyBotInstallationRemoval(ctx, req)
	require.ErrorIs(t, err, ErrBotRemovalBlocked)
	require.Nil(t, got)
	f.Run(true, "goto", "26")
	app := role26Serving(t, f.Pool)
	minimal := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: req.BotId, DecisionId: req.DecisionId}
	receipt, err := (&RoleStore{Pool: app}).RegisterBotGlobalTerminal(globalTerminalContext(t, minimal), minimal)
	require.NoError(t, err)
	require.NotNil(t, receipt.Receipt)
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_removal_operations WHERE operation_id=$1 AND response_bytes IS NULL", req.OperationId).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM member_roles WHERE role_id=$1 AND profile_id=$2", owner, req.ActorProfileId).Scan(&count))
	require.Equal(t, 1, count, "terminal receipt cannot clear Owner cleanup anomaly")
}

func seedOldGlobalForTerminal(t *testing.T, p *pgxpool.Pool, r *rolev1.ApplyBotInstallationRemovalRequest, link bool, corruptHash bool) {
	t.Helper()
	ctx := context.Background()
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	require.NoError(t, err)
	hash := sha256.Sum256(wire)
	if corruptHash {
		hash[0] ^= 1
	}
	_, err = p.Exec(ctx, "INSERT INTO role_bot_actor_bindings(bot_id,actor_profile_id) VALUES($1,$2)", r.BotId, r.ActorProfileId)
	require.NoError(t, err)
	_, err = p.Exec(ctx, "INSERT INTO role_bot_global_removals(bot_id,actor_profile_id,decision_id,decision_sha256) VALUES($1,$2,$3,$4)", r.BotId, r.ActorProfileId, r.DecisionId, r.DecisionSha256)
	require.NoError(t, err)
	if link {
		_, err = p.Exec(ctx, "INSERT INTO role_bot_removal_operations(operation_id,bot_id,actor_profile_id,space_id,installation_id,request_bytes,request_sha256) VALUES($1,$2,$3,$4,$5,$6,$7)", r.OperationId, r.BotId, r.ActorProfileId, r.SpaceId, r.GetNative().InstallationId, wire, hash[:])
		require.NoError(t, err)
	}
}

func TestBotGlobalTerminalStore_RejectsUnqualified25Provenance(t *testing.T) {
	ctx := context.Background()
	f := role26Fixture(t)
	app := role26Serving(t, f.Pool)
	s := &RoleStore{Pool: app}
	for _, tt := range []struct {
		name          string
		link, badHash bool
	}{{"missing accepted source", false, false}, {"corrupt source hash", true, true}} {
		t.Run(tt.name, func(t *testing.T) {
			r := removalRequest()
			r.Purpose = "global_delete"
			seedOldGlobalForTerminal(t, f.Pool, r, tt.link, tt.badHash)
			minimal := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: r.BotId, DecisionId: r.DecisionId}
			result, err := s.RegisterBotGlobalTerminal(globalTerminalContext(t, minimal), minimal)
			require.ErrorIs(t, err, ErrScopeUnavailable)
			require.Nil(t, result)
		})
	}
	r := removalRequest()
	r.Purpose = "global_delete"
	seedOldGlobalForTerminal(t, f.Pool, r, true, false)
	different := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: uuid.NewString(), DecisionId: r.DecisionId}
	_, err := s.RegisterBotGlobalTerminal(globalTerminalContext(t, different), different)
	require.ErrorIs(t, err, ErrBotGlobalTerminalConflict)
	duplicate := removalRequest()
	duplicate.Purpose = "global_delete"
	duplicate.DecisionId = r.DecisionId
	seedOldGlobalForTerminal(t, f.Pool, duplicate, true, false)
	target := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: r.BotId, DecisionId: r.DecisionId}
	_, err = s.RegisterBotGlobalTerminal(globalTerminalContext(t, target), target)
	require.ErrorIs(t, err, ErrScopeUnavailable)
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_global_terminals").Scan(&count))
	require.Zero(t, count)
}

type terminalCancelKey struct{}
type terminalCommitCancel struct{ cancel context.CancelFunc }

func (t terminalCommitCancel) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, terminalCancelKey{}, stringsTerminalInsert(data.SQL))
}
func (t terminalCommitCancel) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if yes, _ := ctx.Value(terminalCancelKey{}).(bool); yes {
		t.cancel()
	}
}
func stringsTerminalInsert(sql string) bool {
	return len(sql) >= len("INSERT INTO public.role_bot_global_terminals") && sql[:len("INSERT INTO public.role_bot_global_terminals")] == "INSERT INTO public.role_bot_global_terminals"
}

func TestBotGlobalTerminalStore_CancelAfterInsertRollsBack(t *testing.T) {
	ctx := context.Background()
	f := role26Fixture(t)
	app := role26Serving(t, f.Pool)
	r := globalTerminalRequest()
	verified := globalTerminalContext(t, r)
	canceled, cancel := context.WithCancel(verified)
	defer cancel()
	cfg := app.Config().Copy()
	cfg.ConnConfig.Tracer = terminalCommitCancel{cancel: cancel}
	faulty, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer faulty.Close()
	result, err := (&RoleStore{Pool: faulty}).RegisterBotGlobalTerminal(canceled, r)
	require.Error(t, err)
	require.Nil(t, result)
	require.True(t, errors.Is(canceled.Err(), context.Canceled))
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_global_terminals").Scan(&count))
	require.Zero(t, count)
	good, err := (&RoleStore{Pool: app}).RegisterBotGlobalTerminal(verified, r)
	require.NoError(t, err)
	require.NotNil(t, good.Receipt)
}

func TestBotGlobalTerminalStore_MissingOrOwnerCatalogFailsClosed(t *testing.T) {
	f := role26Fixture(t)
	r := globalTerminalRequest()
	_, err := (&RoleStore{Pool: f.Pool}).RegisterBotGlobalTerminal(globalTerminalContext(t, r), r)
	require.ErrorIs(t, err, ErrScopeUnavailable)
	app := role26Serving(t, f.Pool)
	var name string
	require.NoError(t, app.QueryRow(context.Background(), "SELECT current_user").Scan(&name))
	_, err = f.Pool.Exec(context.Background(), "REVOKE INSERT ON role_bot_global_terminals FROM "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	_, err = (&RoleStore{Pool: app}).RegisterBotGlobalTerminal(globalTerminalContext(t, r), r)
	require.ErrorIs(t, err, ErrScopeUnavailable)
}

func terminalOldServing(t *testing.T, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	app := role26Serving(t, admin)
	var name string
	require.NoError(t, app.QueryRow(context.Background(), "SELECT current_user").Scan(&name))
	for _, table := range roleBotRemovalCatalog25.Tables {
		if table == "schema_migrations" {
			continue
		}
		_, err := admin.Exec(context.Background(), "GRANT INSERT,UPDATE,DELETE ON "+pgx.Identifier{"public", table}.Sanitize()+" TO "+pgx.Identifier{name}.Sanitize())
		require.NoError(t, err)
	}
	return app
}

type terminalPauseInsert struct {
	entered, release chan struct{}
	once             sync.Once
	match            string
}

func (p *terminalPauseInsert) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	if strings.Contains(q.SQL, p.match) {
		p.once.Do(func() { close(p.entered) })
		select {
		case <-p.release:
		case <-ctx.Done():
		}
	}
	return ctx
}
func (p *terminalPauseInsert) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestBotGlobalTerminalStore_C110OldWinsCrossTableDecision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	f := role26Fixture(t)
	oldApp := terminalOldServing(t, f.Pool)
	newApp := role26Serving(t, f.Pool)
	pause := &terminalPauseInsert{entered: make(chan struct{}), release: make(chan struct{}), match: "INSERT INTO role_bot_global_removals"}
	var release sync.Once
	defer release.Do(func() { close(pause.release) })
	cfg := oldApp.Config().Copy()
	cfg.ConnConfig.Tracer = pause
	oldConn, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer oldConn.Close()
	old := removalRequest()
	old.Purpose = "global_delete"
	oldDone := make(chan error, 1)
	go func() { _, err := (&RoleStore{Pool: oldConn}).ApplyBotInstallationRemoval(ctx, old); oldDone <- err }()
	select {
	case <-pause.entered:
	case err := <-oldDone:
		t.Fatalf("old writer did not reach insertion: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	request := globalTerminalRequest()
	request.DecisionId = old.DecisionId
	newerDone := make(chan error, 1)
	go func() {
		_, err := (&RoleStore{Pool: newApp}).RegisterBotGlobalTerminal(globalTerminalContext(t, request), request)
		newerDone <- err
	}()
	var username string
	require.NoError(t, newApp.QueryRow(ctx, "SELECT current_user").Scan(&username))
	var returned bool
	var newErr error
	require.Eventually(t, func() bool {
		select {
		case newErr = <-newerDone:
			returned = true
			return true
		default:
		}
		var waiting bool
		err := f.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE usename=$1 AND wait_event_type='Lock')", username).Scan(&waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond)
	release.Do(func() { close(pause.release) })
	require.NoError(t, <-oldDone)
	if !returned {
		newErr = <-newerDone
	}
	require.ErrorIs(t, newErr, ErrBotGlobalTerminalConflict)
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_global_terminals").Scan(&count))
	require.Zero(t, count)
}

type terminalPauseCancel struct {
	pause  *terminalPauseInsert
	cancel context.CancelFunc
}

func (p terminalPauseCancel) TraceQueryStart(ctx context.Context, c *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	ctx = p.pause.TraceQueryStart(ctx, c, q)
	return context.WithValue(ctx, terminalCancelKey{}, stringsTerminalInsert(q.SQL))
}
func (p terminalPauseCancel) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	if yes, _ := ctx.Value(terminalCancelKey{}).(bool); yes && p.cancel != nil {
		p.cancel()
	}
}

func TestBotGlobalTerminalStore_C110NewWinsOrRollback(t *testing.T) {
	f := role26Fixture(t)
	for _, rollback := range []bool{false, true} {
		t.Run(fmt.Sprint("rollback=", rollback), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			oldApp := terminalOldServing(t, f.Pool)
			app := role26Serving(t, f.Pool)
			r := globalTerminalRequest()
			verified := globalTerminalContext(t, r)
			newctx, abort := context.WithCancel(verified)
			defer abort()
			pause := &terminalPauseInsert{entered: make(chan struct{}), release: make(chan struct{}), match: "INSERT INTO public.role_bot_global_terminals"}
			var release sync.Once
			defer release.Do(func() { close(pause.release) })
			tracer := terminalPauseCancel{pause: pause}
			if rollback {
				tracer.cancel = abort
			}
			cfg := app.Config().Copy()
			cfg.ConnConfig.Tracer = tracer
			paused, err := pgxpool.NewWithConfig(ctx, cfg)
			require.NoError(t, err)
			defer paused.Close()
			newDone := make(chan error, 1)
			go func() { _, err := (&RoleStore{Pool: paused}).RegisterBotGlobalTerminal(newctx, r); newDone <- err }()
			select {
			case <-pause.entered:
			case err := <-newDone:
				t.Fatalf("new writer did not reach insertion: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			old := removalRequest()
			old.Purpose = "global_delete"
			old.DecisionId = r.DecisionId
			oldDone := make(chan error, 1)
			go func() { _, err := (&RoleStore{Pool: oldApp}).ApplyBotInstallationRemoval(ctx, old); oldDone <- err }()
			var user string
			require.NoError(t, oldApp.QueryRow(ctx, "SELECT current_user").Scan(&user))
			require.Eventually(t, func() bool {
				var waiting bool
				err := f.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE usename=$1 AND wait_event_type='Lock')", user).Scan(&waiting)
				return err == nil && waiting
			}, 10*time.Second, 10*time.Millisecond)
			var count int
			require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_actor_bindings WHERE bot_id=$1", old.BotId).Scan(&count))
			require.Zero(t, count, "blocked before Space/Bot/actor writes")
			release.Do(func() { close(pause.release) })
			newErr, oldErr := <-newDone, <-oldDone
			if rollback {
				require.Error(t, newErr)
				require.NoError(t, oldErr)
			} else {
				require.NoError(t, newErr)
				require.ErrorIs(t, oldErr, ErrBotRemovalConflict)
			}
			require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_global_terminals WHERE decision_id=$1", r.DecisionId).Scan(&count))
			if rollback {
				require.Zero(t, count)
			} else {
				require.Equal(t, 1, count)
			}
			require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM role_bot_global_removals WHERE decision_id=$1", r.DecisionId).Scan(&count))
			if rollback {
				require.Equal(t, 1, count)
			} else {
				require.Zero(t, count)
			}
		})
	}
}

func TestBotGlobalTerminalStore_C110OldOldCrossedAndEqualKeys(t *testing.T) {
	f := role26Fixture(t)
	app := terminalOldServing(t, f.Pool)
	s := &RoleStore{Pool: app}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for _, sameRoot := range []bool{true, false} {
		a, b := removalRequest(), removalRequest()
		a.Purpose = "global_delete"
		b.Purpose = "global_delete"
		if sameRoot {
			b.DecisionId = a.DecisionId
		} else {
			a.OperationId, b.OperationId = a.DecisionId, b.DecisionId
			a.DecisionId, b.DecisionId = b.OperationId, a.OperationId
		}
		errs := make([]error, 2)
		var wg sync.WaitGroup
		for i, r := range []*rolev1.ApplyBotInstallationRemovalRequest{a, b} {
			wg.Add(1)
			go func() { defer wg.Done(); _, errs[i] = s.ApplyBotInstallationRemoval(ctx, r) }()
		}
		wg.Wait()
		if sameRoot {
			require.True(t, (errs[0] == nil && errors.Is(errs[1], ErrBotRemovalConflict)) || (errs[1] == nil && errors.Is(errs[0], ErrBotRemovalConflict)))
		} else {
			require.NoError(t, errs[0])
			require.NoError(t, errs[1])
		}
	}
	equal := removalRequest()
	equal.Purpose = "global_delete"
	equal.OperationId = equal.DecisionId
	_, err := s.ApplyBotInstallationRemoval(ctx, equal)
	require.NoError(t, err)
}

func TestBotGlobalTerminalStore_C110PhysicalKeysAliasesAndNestedDenial(t *testing.T) {
	f := role26Fixture(t)
	ctx := context.Background()
	app := role26Serving(t, f.Pool)
	s := &RoleStore{Pool: app}
	var a, b uuid.UUID
	var hash int32
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT ids[1],ids[2],k FROM (SELECT hashtext(id::text) k,array_agg(id) ids FROM (SELECT md5(i::text)::uuid id FROM generate_series(1,200000)i) q GROUP BY k HAVING count(*)>1) aliases LIMIT 1`).Scan(&a, &b, &hash))
	require.NotEqual(t, a, b)
	keys, err := botGlobalOperationKeys(ctx, f.Pool, []uuid.UUID{a, b, a})
	require.NoError(t, err)
	require.Equal(t, []int32{hash}, keys)
	for _, root := range []uuid.UUID{a, b} {
		r := globalTerminalRequest()
		r.DecisionId = root.String()
		_, err = s.RegisterBotGlobalTerminal(globalTerminalContext(t, r), r)
		require.NoError(t, err, "physical alias never substitutes semantic UUID equality")
	}
	op, decision, space := uuid.New(), uuid.New(), uuid.New()
	keys, err = botGlobalOperationKeys(ctx, f.Pool, []uuid.UUID{op, decision})
	require.NoError(t, err)
	require.True(t, len(keys) == 1 || keys[0] < keys[1])
	require.ErrorIs(t, s.withBotGlobalOperation(ctx, op, decision, &space, func(tx *RoleStore) error {
		var count int
		err := tx.db().QueryRow(ctx, "SELECT count(*) FROM pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory' AND classid=$1::oid", ownershipOperationLockNamespace).Scan(&count)
		require.NoError(t, err)
		require.Equal(t, len(keys), count)
		root := uuid.New()
		return tx.withBotGlobalOperation(ctx, root, root, nil, func(*RoleStore) error { t.Fatal("nested late lock expansion reached callback"); return nil })
	}), ErrScopeUnavailable)
}

func TestBotGlobalTerminalStore_C110PreexistingUnionInconsistency(t *testing.T) {
	f := role26Fixture(t)
	app := role26Serving(t, f.Pool)
	s := &RoleStore{Pool: app}
	r := globalTerminalRequest()
	_, err := s.RegisterBotGlobalTerminal(globalTerminalContext(t, r), r)
	require.NoError(t, err)
	old := removalRequest()
	old.Purpose = "global_delete"
	old.DecisionId = r.DecisionId
	seedOldGlobalForTerminal(t, f.Pool, old, true, false)
	result, err := s.RegisterBotGlobalTerminal(globalTerminalContext(t, r), r)
	require.ErrorIs(t, err, ErrScopeUnavailable)
	require.Nil(t, result)
	var count int
	require.NoError(t, f.Pool.QueryRow(context.Background(), "SELECT count(*) FROM role_bot_global_terminals WHERE decision_id=$1", r.DecisionId).Scan(&count))
	require.Equal(t, 1, count)
}

func TestBotGlobalTerminalStore_OrdinaryUninstallAndComponentFence(t *testing.T) {
	ctx := context.Background()
	f := role26Fixture(t)
	app := terminalOldServing(t, f.Pool)
	s := &RoleStore{Pool: app}
	old := removalRequest()
	old.Purpose = "global_delete"
	original, err := s.ApplyBotInstallationRemoval(ctx, old)
	require.NoError(t, err)
	root := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: old.BotId, DecisionId: old.DecisionId}
	_, err = s.RegisterBotGlobalTerminal(globalTerminalContext(t, root), root)
	require.NoError(t, err)
	replay, err := s.ApplyBotInstallationRemoval(ctx, old)
	require.NoError(t, err)
	require.True(t, proto.Equal(original, replay))
	ordinary := proto.Clone(old).(*rolev1.ApplyBotInstallationRemovalRequest)
	ordinary.OperationId = uuid.NewString()
	ordinary.DecisionId = uuid.NewString()
	ordinary.Purpose = "space_uninstall"
	ordinary.GetNative().InstallationId = uuid.NewString()
	ordinary.GetNative().InstallOperationId = uuid.NewString()
	result, err := s.ApplyBotInstallationRemoval(ctx, ordinary)
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", result.Receipt.Outcome)
	for _, space := range []uuid.UUID{uuid.MustParse(old.SpaceId), uuid.New()} {
		err = s.WithinSpaces(ctx, []uuid.UUID{space}, func(tx *RoleStore) error {
			return tx.CheckBotInstallationRemovalFence(ctx, uuid.MustParse(old.BotId), uuid.MustParse(old.ActorProfileId), space, uuid.New())
		})
		require.ErrorIs(t, err, ErrBotRemovalDenied)
	}
	tx, err := app.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.Error(t, authoritysource.CheckCatalog(ctx, tx, roleBotRemovalCatalog25), "actual unchanged old25 qualified boundary denies26, not a version-label assertion")
}

type terminalOperationTrace struct{ keys []int32 }

func (p *terminalOperationTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	if q.SQL == "SELECT pg_advisory_xact_lock($1::integer,$2::integer)" {
		p.keys = append(p.keys, q.Args[1].(int32))
	}
	return ctx
}
func (*terminalOperationTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestBotGlobalTerminalStore_C110ActualPhysicalAcquisitionOrder(t *testing.T) {
	f := role26Fixture(t)
	app := role26Serving(t, f.Pool)
	ctx := context.Background()
	// Choose opposite signed keys in reverse order, independent of UUID ordering.
	var op, decision uuid.UUID
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT id FROM (SELECT md5(i::text)::uuid id FROM generate_series(1,100)i) q WHERE hashtext(id::text)>0 LIMIT 1`).Scan(&op))
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT id FROM (SELECT md5(i::text)::uuid id FROM generate_series(101,200)i) q WHERE hashtext(id::text)<0 LIMIT 1`).Scan(&decision))
	keys, err := botGlobalOperationKeys(ctx, f.Pool, []uuid.UUID{op, decision})
	require.NoError(t, err)
	require.Len(t, keys, 2)
	trace := &terminalOperationTrace{}
	cfg := app.Config().Copy()
	cfg.ConnConfig.Tracer = trace
	observed, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	defer observed.Close()
	space := uuid.New()
	require.NoError(t, (&RoleStore{Pool: observed}).withBotGlobalOperation(ctx, op, decision, &space, func(*RoleStore) error { return nil }))
	require.Equal(t, keys, trace.keys, "actual sequential SQL acquisitions use signed physical order")
}

func TestBotGlobalTerminalStore_ComponentFenceHoldsBotUntilCommit(t *testing.T) {
	f := role26Fixture(t)
	app := terminalOldServing(t, f.Pool)
	s := &RoleStore{Pool: app}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	r := globalTerminalRequest()
	bot, actor, space := uuid.MustParse(r.BotId), uuid.New(), uuid.New()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	guardDone := make(chan error, 1)
	go func() {
		guardDone <- s.WithinSpaces(ctx, []uuid.UUID{space}, func(tx *RoleStore) error {
			if err := tx.CheckBotInstallationRemovalFence(ctx, bot, actor, space, uuid.New()); err != nil {
				return err
			}
			close(entered)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	select {
	case <-entered:
	case err := <-guardDone:
		t.Fatalf("component guard did not reach held Bot lock: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	rootApp := role26Serving(t, f.Pool)
	rootDone := make(chan error, 1)
	go func() {
		_, err := (&RoleStore{Pool: rootApp}).RegisterBotGlobalTerminal(globalTerminalContext(t, r), r)
		rootDone <- err
	}()
	var user string
	require.NoError(t, rootApp.QueryRow(ctx, "SELECT current_user").Scan(&user))
	require.Eventually(t, func() bool {
		var waiting bool
		err := f.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE usename=$1 AND wait_event_type='Lock')", user).Scan(&waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond)
	once.Do(func() { close(release) })
	require.NoError(t, <-guardDone)
	require.NoError(t, <-rootDone)
	require.ErrorIs(t, s.WithinSpaces(ctx, []uuid.UUID{space}, func(tx *RoleStore) error {
		return tx.CheckBotInstallationRemovalFence(ctx, bot, actor, space, uuid.New())
	}), ErrBotRemovalDenied, "later component operation sees committed terminal under the same Bot lock")
}
