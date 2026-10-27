package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"net"
	"os"
	"time"
	"voice/backend/pkg/socialprincipal"
	"voice/backend/user/testfixture"
)

func main() {
	var input struct{ DSN, CertPath, KeyPath, JWKSURL, ReplayAddr string }
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		panic(err)
	}
	roots := x509.NewCertPool()
	pem, err := os.ReadFile(input.CertPath)
	if err != nil {
		panic(err)
	}
	if !roots.AppendCertsFromPEM(pem) {
		panic("invalid fixture CA")
	}
	cfg := socialprincipal.Config{Target: "user", Capability: "messaging", JWKSURLs: map[string]string{"messaging": input.JWKSURL}, RefreshAfter: 30 * time.Second, HardExpiry: 2 * time.Minute, UnknownKIDCooldown: 5 * time.Second, ReplayAddr: input.ReplayAddr, JWKSCAFile: input.CertPath, TLSCertFile: input.CertPath, TLSKeyFile: input.KeyPath, ListenAddr: "127.0.0.1:0", ClientCAFile: input.CertPath, ClientCAs: roots, JWKSClientCertFile: input.CertPath, JWKSClientKeyFile: input.KeyPath}
	runtime, err := socialprincipal.New(context.Background(), cfg)
	if err != nil {
		panic(err)
	}
	defer runtime.Close()
	pool, err := pgxpool.New(context.Background(), input.DSN)
	if err != nil {
		panic(err)
	}
	defer pool.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	server := grpc.NewServer(runtime.ServerOptions()...)
	testfixture.RegisterMessagingDeletionStates(server, pool)
	fmt.Println(listener.Addr().String())
	if err := server.Serve(listener); err != nil {
		panic(err)
	}
}
