package main

import (
	"context"
	"google.golang.org/grpc"
	"voice/backend/pkg/authoritysource"
)

// The source is registered exclusively on this dedicated mTLS serving set.
// Public HTTP, workload-HMAC and Messaging listeners never expose it.
func newGISAuthorityServer(ctx context.Context, cfg authoritysource.RuntimeConfig, reader authoritysource.SchemaReader) (*grpc.Server, *authoritysource.Runtime, error) {
	runtime, err := authoritysource.NewRuntime(ctx, cfg, reader)
	if err != nil {
		return nil, nil, err
	}
	server := grpc.NewServer(runtime.ServerOptions()...)
	runtime.Register(server)
	return server, runtime, nil
}
