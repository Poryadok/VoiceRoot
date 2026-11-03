// Command story_media_listener is a subprocess fixture for the Story consumer
// test. Keeping File in its own process preserves each service's protobuf module.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/principalruntime"
	"voice/backend/file/internal/store"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var input struct {
		DSN    string
		Config principalruntime.Config
	}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		return err
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, input.DSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}
	runtime, err := principalruntime.New(ctx, input.Config)
	if err != nil {
		return err
	}
	defer runtime.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	server := grpc.NewServer(runtime.ServerOptions()...)
	filev1.RegisterFileServiceServer(server, grpcsvc.New(grpcsvc.Deps{Files: store.NewFilesStore(pool)}))
	defer server.Stop()
	go func() { _ = server.Serve(listener) }()
	if err := json.NewEncoder(os.Stdout).Encode(listener.Addr().String()); err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, os.Stdin)
	return err
}
