package main

import (
	"context"
	"google.golang.org/grpc"
	"voice/backend/pkg/authoritysource"
)

// The source has its own serving set and mTLS credentials. It is never
// registered on Space's legacy, privacy, GIS, or Gateway lifecycle listeners.
func newSpaceAuthorityServer(ctx context.Context, shared []grpc.ServerOption, cfg authoritysource.RuntimeConfig, reader authoritysource.SchemaReader) (*grpc.Server, *authoritysource.Runtime, error) {
	runtime, err := authoritysource.NewRuntime(ctx, cfg, reader)
	if err != nil {
		return nil, nil, err
	}
	options := append([]grpc.ServerOption{}, shared...)
	options = append(options, runtime.ServerOptions()...)
	server := grpc.NewServer(options...)
	runtime.Register(server)
	return server, runtime, nil
}
