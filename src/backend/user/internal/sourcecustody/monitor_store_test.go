package sourcecustody

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestMonitorStoreExactRetriesAndIndependentStop(t *testing.T) {
	ctx := context.Background()
	pool := custodyDatabase(t, ctx)
	_, err := pool.Exec(ctx, `CREATE TABLE public.user_custody_monitor_probe(challenge bytea NOT NULL,sequence bigint NOT NULL,decision_digest bytea NOT NULL,nonce bytea NOT NULL,PRIMARY KEY(challenge,sequence),UNIQUE(challenge,sequence,nonce));
CREATE TABLE public.user_custody_monitor_response(challenge bytea NOT NULL,sequence bigint NOT NULL,kind smallint NOT NULL,nonce bytea NOT NULL,incarnation uuid NOT NULL,signature bytea NOT NULL,PRIMARY KEY(challenge,sequence,kind),FOREIGN KEY(challenge,sequence,nonce) REFERENCES public.user_custody_monitor_probe(challenge,sequence,nonce));
CREATE UNIQUE INDEX monitor_one_stop ON public.user_custody_monitor_response(challenge) WHERE kind=3`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE ROLE monitor_controller LOGIN PASSWORD 'fixture_controller';
CREATE ROLE monitor_responder LOGIN PASSWORD 'fixture_responder';
GRANT USAGE ON SCHEMA public TO monitor_controller,monitor_responder;
GRANT SELECT,INSERT ON public.user_custody_monitor_probe TO monitor_controller;
GRANT SELECT ON public.user_custody_monitor_response TO monitor_controller;
GRANT SELECT ON public.user_custody_monitor_probe TO monitor_responder;
GRANT SELECT,INSERT ON public.user_custody_monitor_response TO monitor_responder`)
	require.NoError(t, err)
	rolePool := func(user, password string) *pgxpool.Pool {
		cfg := pool.Config().Copy()
		cfg.ConnConfig.User, cfg.ConnConfig.Password = user, password
		p, poolErr := pgxpool.NewWithConfig(ctx, cfg)
		require.NoError(t, poolErr)
		t.Cleanup(p.Close)
		require.NoError(t, p.Ping(ctx))
		return p
	}
	controllerPool := rolePool("monitor_controller", "fixture_controller")
	responderPool := rolePool("monitor_responder", "fixture_responder")
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	store := &postgresProbeStore{pool: controllerPool}
	responder := &postgresResponseStore{pool: responderPool}
	p := supervisionProbe{Challenge: supervisionNonce(1), DecisionDigest: supervisionNonce(2), Sequence: 1, Nonce: supervisionNonce(3)}
	require.NoError(t, store.PublishProbe(ctx, p))
	require.NoError(t, store.PublishProbe(ctx, p))
	readProbe, found, err := responder.ReadProbe(ctx, p.Challenge, p.DecisionDigest, p.Sequence)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, p, readProbe)
	_, found, err = responder.ReadProbe(ctx, p.Challenge, p.DecisionDigest, 2)
	require.NoError(t, err)
	require.False(t, found)
	_, _, err = responder.ReadProbe(ctx, p.Challenge, supervisionNonce(8), p.Sequence)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO public.user_custody_monitor_probe(challenge,sequence,decision_digest,nonce) VALUES($1,3,$2,$3)`, p.Challenge[:], p.DecisionDigest[:], make([]byte, 31))
	require.NoError(t, err)
	_, _, err = responder.ReadProbe(ctx, p.Challenge, p.DecisionDigest, 3)
	require.Error(t, err, "malformed durable rows must not become probes")
	conflict := p
	conflict.Nonce = supervisionNonce(4)
	require.Error(t, store.PublishProbe(ctx, conflict))
	id := uuid.New()
	armed := signedProbeResponse(t, p, id, MonitorArmed, private)
	stop := signedProbeResponse(t, p, id, MonitorStop, private)
	require.NoError(t, responder.InsertExactResponse(ctx, armed))
	require.NoError(t, responder.InsertExactResponse(ctx, armed))
	wrongDigest := armed
	wrongDigest.DecisionDigest = supervisionNonce(8)
	require.Error(t, responder.InsertExactResponse(ctx, wrongDigest), "a retry must match the signed probe digest")
	other := signedProbeResponse(t, p, uuid.New(), MonitorArmed, private)
	require.Error(t, responder.InsertExactResponse(ctx, other))
	require.NoError(t, responder.InsertExactResponse(ctx, stop))
	p2 := p
	p2.Sequence, p2.Nonce = 2, supervisionNonce(5)
	require.NoError(t, store.PublishProbe(ctx, p2))
	wrongProbe := p2
	wrongProbe.DecisionDigest = supervisionNonce(8)
	require.Error(t, responder.InsertExactResponse(ctx, signedProbeResponse(t, wrongProbe, id, MonitorHeartbeat, private)))
	heartbeat := signedProbeResponse(t, p2, id, MonitorHeartbeat, private)
	results := make(chan error, 8)
	for i := 0; i < cap(results); i++ {
		go func() { results <- responder.InsertExactResponse(ctx, heartbeat) }()
	}
	for i := 0; i < cap(results); i++ {
		require.NoError(t, <-results)
	}
	assertDenied := func(err error) {
		t.Helper()
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "42501", pgErr.Code)
	}
	_, err = controllerPool.Exec(ctx, `DELETE FROM public.user_custody_monitor_response`)
	assertDenied(err)
	_, err = controllerPool.Exec(ctx, `INSERT INTO public.user_custody_monitor_response SELECT * FROM public.user_custody_monitor_response LIMIT 1`)
	assertDenied(err)
	_, err = responderPool.Exec(ctx, `UPDATE public.user_custody_monitor_probe SET sequence=2`)
	assertDenied(err)
	_, err = responderPool.Exec(ctx, `DELETE FROM public.user_custody_monitor_response`)
	assertDenied(err)
	_, err = responderPool.Exec(ctx, `UPDATE public.user_custody_monitor_response SET kind=3`)
	assertDenied(err)
	rows, err := store.ReadResponses(ctx, p, supervisionProbe{})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.NoError(t, VerifyMonitorResponse(row, public))
	}
	bad := p
	bad.DecisionDigest = supervisionNonce(9)
	_, err = store.ReadResponses(ctx, bad, supervisionProbe{})
	require.Error(t, err)
}
