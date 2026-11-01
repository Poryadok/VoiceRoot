// Command sourcecustody starts admission closed or performs an operator operation.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"voice/backend/user/internal/sourcecustody"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	if len(os.Args) == 2 && os.Args[1] == "sign-open" {
		err = signOpen()
	} else if len(os.Args) == 2 && os.Args[1] == "fence-current" {
		var cfg sourcecustody.FenceConfig
		cfg, err = sourcecustody.LoadFenceConfig(os.LookupEnv)
		if err == nil {
			err = sourcecustody.RunFence(ctx, cfg)
		}
	} else if len(os.Args) == 2 && os.Args[1] == "create-closed" {
		var cfg sourcecustody.CreationConfig
		cfg, err = sourcecustody.LoadCreationConfig(os.LookupEnv)
		if err == nil {
			err = sourcecustody.RunCreation(ctx, cfg)
		}
	} else if len(os.Args) == 2 && (os.Args[1] == "inspect-writer-role" || os.Args[1] == "recover-activation" || os.Args[1] == "watch-activation") {
		var cfg sourcecustody.WriterConfig
		cfg, err = sourcecustody.LoadWriterConfig(os.LookupEnv)
		if err == nil {
			switch os.Args[1] {
			case "watch-activation":
				err = sourcecustody.WatchActivation(ctx, cfg, func() { fmt.Println("WATCHING") })
			case "recover-activation":
				err = sourcecustody.RunActivationRecovery(ctx, cfg)
			default:
				err = sourcecustody.RunWriterConfinement(ctx, cfg)
			}
		}
	} else if len(os.Args) == 2 && os.Args[1] == "migrate-existing" {
		var creation sourcecustody.CreationConfig
		var recovery sourcecustody.RecoveryConfig
		creation, err = sourcecustody.LoadCreationConfig(os.LookupEnv)
		if err == nil {
			recovery, err = sourcecustody.LoadRecoveryConfig(os.LookupEnv)
		}
		if err == nil {
			err = sourcecustody.RunExistingMigration(ctx, creation, recovery)
		}
	} else if len(os.Args) == 2 && os.Args[1] == "check-existing-identity" {
		var cfg sourcecustody.CheckedSourceConfig
		cfg, err = sourcecustody.LoadCheckedSourceConfig(os.LookupEnv)
		if err == nil {
			err = sourcecustody.RunCheckedSource(ctx, cfg)
		}
	} else if len(os.Args) == 2 && (os.Args[1] == "recover-initialization" || os.Args[1] == "quarantine-existing") {
		var cfg sourcecustody.RecoveryConfig
		cfg, err = sourcecustody.LoadRecoveryConfig(os.LookupEnv)
		if err == nil {
			if os.Args[1] == "quarantine-existing" {
				err = sourcecustody.RunQuarantine(ctx, cfg)
			} else {
				err = sourcecustody.RunRecovery(ctx, cfg)
			}
		}
	} else if len(os.Args) == 1 {
		var cfg sourcecustody.ServerConfig
		cfg, err = sourcecustody.LoadServerConfig(os.LookupEnv)
		if err == nil {
			err = sourcecustody.ServeClosed(ctx, cfg, func(addr string) { fmt.Printf("READY %s\n", addr) })
		}
	} else {
		err = sourcecustody.ErrUnavailable
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "source custody unavailable")
		os.Exit(1)
	}
}

// signOpen is an offline custodian operation. A successful signature only
// expresses the custodian's decision; it does not verify the evidence itself.
func signOpen() error {
	path, ok := os.LookupEnv("CUSTODY_OPERATOR_SIGNER_PRIVATE_KEY_FILE")
	if !ok || path == "" {
		return sourcecustody.ErrUnavailable
	}
	file, err := os.Open(path)
	if err != nil {
		return sourcecustody.ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(file, 4097))
	closeErr := file.Close()
	if err != nil || closeErr != nil || len(raw) == 0 || len(raw) > 4096 {
		return sourcecustody.ErrUnavailable
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "PRIVATE KEY" || len(rest) != 0 {
		return sourcecustody.ErrUnavailable
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok {
		return sourcecustody.ErrUnavailable
	}
	decoder := json.NewDecoder(io.LimitReader(os.Stdin, 16385))
	decoder.DisallowUnknownFields()
	var claim sourcecustody.OpenAttestation
	if decoder.Decode(&claim) != nil || decoder.Decode(new(any)) != io.EOF {
		return sourcecustody.ErrUnavailable
	}
	signed, err := sourcecustody.SignOpenAttestation(claim, key)
	if err != nil {
		return sourcecustody.ErrUnavailable
	}
	return json.NewEncoder(os.Stdout).Encode(signed)
}
