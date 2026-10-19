package squad

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/grpc"
	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/pkg/grpcclient"
)

// LoadProtectedPartySnapshotReader constructs the dedicated mTLS Voice client.
// Configuration presence is not a capability or a native startup qualification.
// The native composition must require this client and its complete dependencies.
func LoadProtectedPartySnapshotReader(getenv func(string) string) (*partysource.GRPCClient, func() error, bool, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	names := []string{
		"MATCHMAKING_VOICE_PARTY_SNAPSHOT_GRPC_ADDR", "MATCHMAKING_VOICE_PARTY_SNAPSHOT_CA_FILE",
		"MATCHMAKING_VOICE_PARTY_SNAPSHOT_CLIENT_CERT_FILE", "MATCHMAKING_VOICE_PARTY_SNAPSHOT_CLIENT_KEY_FILE",
		"MATCHMAKING_VOICE_PARTY_SNAPSHOT_SERVER_NAME", "MATCHMAKING_PRINCIPAL_SIGNING_KEYS_DIR", "MATCHMAKING_PRINCIPAL_ACTIVE_KID",
	}
	values := make([]string, len(names))
	configured := false
	for i, name := range names {
		values[i] = strings.TrimSpace(getenv(name))
		// Shared signing keys may already be installed for MatchSquad. They do
		// not implicitly enable a new source client in a pre-cut executable.
		if i < 5 && values[i] != "" {
			configured = true
		}
	}
	if !configured {
		return nil, func() error { return nil }, false, nil
	}
	for i, value := range values {
		if value == "" {
			return nil, nil, true, fmt.Errorf("required party snapshot configuration %s is missing", names[i])
		}
	}
	issuer, err := loadMatchmakingIssuer(values[5], values[6])
	if err != nil {
		return nil, nil, true, err
	}
	certificate, err := tls.LoadX509KeyPair(values[2], values[3])
	if err != nil {
		return nil, nil, true, errors.New("party snapshot client TLS identity is invalid")
	}
	transport, err := providerTLSCredentials(values[1], values[4], certificate)
	if err != nil {
		return nil, nil, true, errors.New("party snapshot server TLS trust is invalid")
	}
	conn, err := grpc.NewClient(grpcclient.DialTarget(values[0]), grpc.WithTransportCredentials(transport))
	if err != nil {
		return nil, nil, true, errors.New("party snapshot mTLS client unavailable")
	}
	return &partysource.GRPCClient{Client: callsv1.NewVoiceServiceClient(conn), Issuer: issuer}, conn.Close, true, nil
}
