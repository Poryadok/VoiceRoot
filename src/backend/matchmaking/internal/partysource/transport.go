package partysource

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"time"
	callsv1 "voice.app/voice/calls/v1"
)

// NewGRPCClient retains the actual connection built by the strict TLS factory.
// Connection readiness is transport data, never native source/coverage proof.
func NewGRPCClient(conn *grpc.ClientConn, issuer ServiceIssuer) *GRPCClient {
	if conn == nil || issuer == nil {
		return nil
	}
	return &GRPCClient{Client: callsv1.NewVoiceServiceClient(conn), Issuer: issuer, connection: conn}
}

func (c *GRPCClient) CheckTransport(ctx context.Context) error {
	if c == nil {
		return ErrUnknown
	}
	return WaitForTransport(ctx, c.connection)
}

func WaitForTransport(ctx context.Context, conn *grpc.ClientConn) error {
	if ctx == nil || ctx.Err() != nil || conn == nil {
		return ErrUnknown
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn.Connect()
	for bounded.Err() == nil {
		state := conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if state == connectivity.Shutdown || !conn.WaitForStateChange(bounded, state) {
			return ErrUnknown
		}
	}
	return ErrUnknown
}
