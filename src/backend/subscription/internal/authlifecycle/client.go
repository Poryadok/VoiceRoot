// Package authlifecycle observes Auth-owned inputs. Neither observation nor
// durable ingestion qualifies Subscription CURRENT authority or a paid grant.
package authlifecycle

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

var ErrUnavailable = errors.New("auth lifecycle source unavailable")
var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

type Config struct {
	Address, ServerName, CAFile, CertificateFile, KeyFile, ClientURI string
	SigningKeyFile, SigningKeyID, Generation                         string
}

type Client struct {
	connection *grpc.ClientConn
	rpc        authv1.AuthSubscriptionLifecycleServiceClient
	issuer     *principal.Issuer
	generation string
}

// Observation has no public constructor. It is minted only by the authenticated
// client and carries the initiating generation/request/deadline into ingestion.
type Observation struct {
	generation, account, requestID string
	requestHash, snapshotHash      [32]byte
	snapshot                       *authv1.ObserveSubscriptionAccountResponse
	payload                        []byte
	deadline                       time.Time
	minimum                        uint64
}

func (o *Observation) Snapshot() *authv1.ObserveSubscriptionAccountResponse {
	if o == nil || o.snapshot == nil {
		return nil
	}
	return proto.Clone(o.snapshot).(*authv1.ObserveSubscriptionAccountResponse)
}

func validID(text string) bool {
	id, err := uuid.Parse(text)
	return err == nil && id != uuid.Nil && id.String() == text
}

// New never borrows an ordinary/public Auth endpoint or system public roots.
// Explicit service trust, client identity, signer and installed generation are required.
func New(config Config) (*Client, error) {
	if !validID(config.Generation) || !keyIDPattern.MatchString(config.SigningKeyID) || config.ServerName == "" || strings.ContainsAny(config.ServerName, "* /\\") {
		return nil, ErrUnavailable
	}
	if _, _, err := net.SplitHostPort(config.Address); err != nil {
		return nil, ErrUnavailable
	}
	peer, err := url.Parse(config.ClientURI)
	if err != nil || !peer.IsAbs() || peer.User != nil || peer.RawQuery != "" || peer.Fragment != "" || strings.Contains(config.ClientURI, "*") {
		return nil, ErrUnavailable
	}
	ca, err := os.ReadFile(config.CAFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(ca) {
		return nil, ErrUnavailable
	}
	certificate, err := tls.LoadX509KeyPair(config.CertificateFile, config.KeyFile)
	if err != nil || len(certificate.Certificate) == 0 {
		return nil, ErrUnavailable
	}
	leaf, err := x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		return nil, ErrUnavailable
	}
	matched := false
	for _, uri := range leaf.URIs {
		if uri.String() == config.ClientURI {
			matched = true
		}
	}
	if !matched {
		return nil, ErrUnavailable
	}
	file, err := os.Open(config.SigningKeyFile)
	if err != nil {
		return nil, ErrUnavailable
	}
	encoded, readErr := io.ReadAll(io.LimitReader(file, 32769))
	closeErr := file.Close()
	if readErr != nil || closeErr != nil || len(encoded) > 32768 {
		return nil, ErrUnavailable
	}
	block, rest := pem.Decode(encoded)
	if block == nil || block.Type != "PRIVATE KEY" || strings.TrimSpace(string(rest)) != "" {
		return nil, ErrUnavailable
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, ErrUnavailable
	}
	key, ok := parsed.(*rsa.PrivateKey)
	if !ok || key.N.BitLen() < 2048 || key.Validate() != nil {
		return nil, ErrUnavailable
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "subscription", KeyID: config.SigningKeyID, PrivateKey: key})
	if err != nil {
		return nil, ErrUnavailable
	}
	connection, err := grpc.NewClient(config.Address, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: config.ServerName, Certificates: []tls.Certificate{certificate}})), grpc.WithDisableRetry(), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1<<20)))
	if err != nil {
		return nil, ErrUnavailable
	}
	return &Client{connection: connection, rpc: authv1.NewAuthSubscriptionLifecycleServiceClient(connection), issuer: issuer, generation: config.Generation}, nil
}

func (c *Client) Close() error {
	if c == nil || c.connection == nil {
		return nil
	}
	return c.connection.Close()
}

// Observe performs exactly one request, before any local authority transaction.
// Its deadline is the earlier of 500ms and the caller's remaining deadline.
func (c *Client) Observe(ctx context.Context, account string, minimum uint64) (*Observation, error) {
	if c == nil || c.rpc == nil || c.issuer == nil || !validID(account) || minimum > math.MaxInt64 {
		return nil, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if ctx.Err() != nil {
		return nil, ErrUnavailable
	}
	started := time.Now()
	deadline, _ := ctx.Deadline()
	request := &authv1.ObserveSubscriptionAccountRequest{AccountId: account, MinimumLifecycleRevision: minimum}
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(request)
	if err != nil {
		return nil, ErrUnavailable
	}
	hash := sha256.Sum256(body)
	requestID := uuid.NewString()
	token, err := c.issuer.IssueService(principal.ServiceInput{Audience: "auth", RPC: authv1.AuthSubscriptionLifecycleService_ObserveSubscriptionAccount_FullMethodName, RequestID: requestID, RequestHash: fmt.Sprintf("sha256:%x", hash)})
	if err != nil {
		return nil, ErrUnavailable
	}
	// Replace incoming metadata, including any delegated credentials/raw hints.
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	response, err := c.rpc.ObserveSubscriptionAccount(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("%w: Auth status %s", ErrUnavailable, status.Code(err))
	}
	if ctx.Err() != nil || !validate(response, account, minimum, started, time.Now()) {
		return nil, ErrUnavailable
	}
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil || ctx.Err() != nil {
		return nil, ErrUnavailable
	}
	return &Observation{generation: c.generation, account: account, requestID: requestID, requestHash: hash, snapshotHash: sha256.Sum256(payload), snapshot: proto.Clone(response).(*authv1.ObserveSubscriptionAccountResponse), payload: payload, deadline: deadline, minimum: minimum}, nil
}

func known(message proto.Message) bool {
	return message != nil && message.ProtoReflect().IsValid() && len(message.ProtoReflect().GetUnknown()) == 0
}
func stamp(value *timestamppb.Timestamp) bool {
	return value != nil && known(value) && value.CheckValid() == nil
}

func validate(s *authv1.ObserveSubscriptionAccountResponse, account string, minimum uint64, started, received time.Time) bool {
	if s == nil || !known(s) || s.ProtocolVersion != 1 || s.AccountId != account || !validID(account) || s.SessionEpoch == 0 || s.SessionEpoch > math.MaxInt64 || s.SecurityRevision == 0 || s.SecurityRevision > math.MaxInt64 || !stamp(s.ObservedAt) {
		return false
	}
	if s.AccountType != "regular" && s.AccountType != "guest" {
		return false
	}
	if s.AccountStatus != "active" && s.AccountStatus != "suspended" && s.AccountStatus != "deleted" {
		return false
	}
	if s.ObservedAt.AsTime().Before(started.Add(-5*time.Second)) || s.ObservedAt.AsTime().After(received.Add(5*time.Second)) {
		return false
	}
	if (s.AccountStatus == "deleted") != (s.DeletedAt != nil) || (s.DeletedAt != nil && !stamp(s.DeletedAt)) {
		return false
	}
	if s.CaptureCoverage == authv1.SubscriptionAccountCaptureCoverage_SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_UNRECORDED {
		return minimum == 0 && s.Head == nil && len(s.Facts) == 0
	}
	if s.CaptureCoverage != authv1.SubscriptionAccountCaptureCoverage_SUBSCRIPTION_ACCOUNT_CAPTURE_COVERAGE_CAPTURED || len(s.Facts) == 0 || len(s.Facts) > 256 || s.Head == nil || !proto.Equal(s.Head, s.Facts[len(s.Facts)-1]) || s.Head.SourceRevision < minimum {
		return false
	}
	seen := map[string]bool{}
	var previous *authv1.SubscriptionAccountLifecycleFact
	for i, f := range s.Facts {
		if f == nil || !known(f) || f.AccountId != account || f.SourceRevision != uint64(i+1) || !validID(f.EventId) || !validID(f.DeletionCycleId) || seen[f.EventId] || f.SessionEpoch == 0 || f.SessionEpoch > s.SessionEpoch || !stamp(f.ScheduledAt) || !stamp(f.PurgeAt) || !stamp(f.EffectiveAt) || !stamp(f.CapturedAt) {
			return false
		}
		seen[f.EventId] = true
		if f.CapturedAt.AsTime().After(s.ObservedAt.AsTime()) || f.EffectiveAt.AsTime().After(s.ObservedAt.AsTime()) {
			return false
		}
		if !f.PurgeAt.AsTime().Equal(f.ScheduledAt.AsTime().Add(720 * time.Hour)) {
			return false
		}
		switch f.Kind {
		case authv1.SubscriptionAccountLifecycleKind_SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_DELETE_SCHEDULED:
			if f.EventId != f.DeletionCycleId || !proto.Equal(f.EffectiveAt, f.ScheduledAt) || (previous != nil && previous.Kind != authv1.SubscriptionAccountLifecycleKind_SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED) {
				return false
			}
		case authv1.SubscriptionAccountLifecycleKind_SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED:
			if previous == nil || previous.Kind != authv1.SubscriptionAccountLifecycleKind_SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_DELETE_SCHEDULED || previous.DeletionCycleId != f.DeletionCycleId || !proto.Equal(previous.ScheduledAt, f.ScheduledAt) || !proto.Equal(previous.PurgeAt, f.PurgeAt) {
				return false
			}
		default:
			return false
		}
		previous = f
	}
	if s.AccountStatus == "deleted" {
		return s.Head.Kind == authv1.SubscriptionAccountLifecycleKind_SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_DELETE_SCHEDULED && proto.Equal(s.DeletedAt, s.Head.ScheduledAt)
	}
	return s.Head.Kind == authv1.SubscriptionAccountLifecycleKind_SUBSCRIPTION_ACCOUNT_LIFECYCLE_KIND_RESTORE_REQUESTED
}
