package reportthreshold

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/moderation/internal/automod"
	"voice/backend/moderation/internal/store"
	"voice/backend/pkg/automodclient"
	"voice/backend/pkg/principal"
)

const configPrefix = "MODERATION_REPORT_THRESHOLD_"

type Config struct{ UserAddr, ServerName, CAFile, CertFile, KeyFile string }

func LoadConfig() (*Config, error) {
	fields := []string{"USER_ADDR", "TLS_SERVER_NAME", "CA_FILE", "TLS_CERT_FILE", "TLS_KEY_FILE"}
	values := make([]string, len(fields))
	set := false
	for i, field := range fields {
		raw, exists := os.LookupEnv(configPrefix + field)
		set = set || exists
		values[i] = strings.TrimSpace(raw)
	}
	if !set {
		return nil, nil
	}
	for _, value := range values {
		if value == "" {
			return nil, errors.New("complete report threshold User authority configuration required")
		}
	}
	host, port, err := net.SplitHostPort(values[0])
	if err != nil || host == "" || strings.ContainsAny(host, "/@ ") {
		return nil, errors.New("invalid report threshold User address")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number <= 0 || number > 65535 {
		return nil, errors.New("invalid report threshold User port")
	}
	return &Config{UserAddr: values[0], ServerName: values[1], CAFile: values[2], CertFile: values[3], KeyFile: values[4]}, nil
}

type Runtime struct {
	Store      *store.ReportThresholdStore
	Worker     *Worker
	Configured bool
	user       *grpc.ClientConn
	natsURL    string
	publisher  *Publisher
	cancel     context.CancelFunc
	wait       sync.WaitGroup
	once       sync.Once
}

// NewRuntime reuses BE-045's one issuer, subject client and lifecycle store.
// Hooks remain available in manual mode so old cases still close and erase.
// Authority and NATS outages are retried after startup, outside report intake.
func NewRuntime(pool *pgxpool.Pool, issuer *principal.Issuer, auth *automodclient.AuthClient, subjects *automod.Store, natsURL string) (result *Runtime, returnErr error) {
	if pool == nil || subjects == nil {
		return nil, errors.New("report threshold owning stores required")
	}
	result = &Runtime{Store: &store.ReportThresholdStore{Pool: pool, Sanctions: &store.SanctionStore{Pool: pool}}, natsURL: natsURL}
	result.Store.BindLiveSubjectTx = func(ctx context.Context, tx pgx.Tx, account, subject uuid.UUID, revision uint64) error {
		if err := subjects.BindLiveSubjectTx(ctx, tx, account, subject, int64(revision)); err != nil {
			return err
		}
		return subjects.RequireLiveSubjectTx(ctx, tx, account, subject, int64(revision))
	}
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	if config == nil {
		return result, nil
	}
	if issuer == nil || auth == nil || strings.TrimSpace(natsURL) == "" {
		return nil, errors.New("report threshold shared signed Auth client and NATS required")
	}
	caPEM, err := os.ReadFile(config.CAFile)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("invalid report threshold User CA")
	}
	identity, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, err
	}
	owned := result
	defer func() {
		if returnErr != nil {
			owned.Close()
		}
	}()
	result.user, err = grpc.NewClient(config.UserAddr, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: config.ServerName, Certificates: []tls.Certificate{identity}})))
	if err != nil {
		return nil, err
	}
	result.Worker = &Worker{Store: result.Store, Audience: AudienceClient{User: userv1.NewUserServiceClient(result.user), Issuer: issuer}, ResolveSubject: auth.ResolveSubject, Events: Events}
	result.Configured = true
	return result, nil
}

func (r *Runtime) Start(ctx context.Context, logger *slog.Logger) {
	if r == nil || !r.Configured {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	r.once.Do(func() {
		workCtx, cancel := context.WithCancel(ctx)
		r.cancel = cancel
		r.wait.Go(func() {
			r.loop(workCtx, logger, "evaluation", func(ctx context.Context) (bool, error) { return r.Worker.EvaluateOne(ctx) })
		})
		r.wait.Go(func() {
			r.loop(workCtx, logger, "publication", func(ctx context.Context) (bool, error) { return r.Store.DeliverEvent(ctx, r.publish) })
		})
	})
}
func (r *Runtime) loop(ctx context.Context, logger *slog.Logger, phase string, work func(context.Context) (bool, error)) {
	for ctx.Err() == nil {
		worked, err := work(ctx)
		if err != nil && ctx.Err() == nil {
			logger.Warn("report threshold work deferred", slog.String("phase", phase))
		}
		if worked && err == nil {
			continue
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
func (r *Runtime) publish(ctx context.Context, event store.ReportThresholdEvent) error {
	if r.publisher == nil {
		publisher, err := NewPublisher(r.natsURL)
		if err != nil {
			return err
		}
		r.publisher = publisher
	}
	return r.publisher.Publish(ctx, event)
}
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	if r.cancel != nil {
		r.cancel()
		r.wait.Wait()
	}
	if r.publisher != nil {
		r.publisher.Close()
	}
	if r.user != nil {
		_ = r.user.Close()
	}
}
