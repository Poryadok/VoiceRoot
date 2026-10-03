package main

import (
	"context"
	"google.golang.org/grpc"
	"voice/backend/pkg/authoritysource"
)

// This source-only serving set never exposes User methods or receipts and is
// never registered on the ordinary, privacy, File, Search or Auth listeners.
func newUserAuthorityServer(ctx context.Context, shared []grpc.ServerOption, cfg authoritysource.RuntimeConfig, reader authoritysource.SchemaReader) (*grpc.Server, *authoritysource.Runtime, error) {
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
