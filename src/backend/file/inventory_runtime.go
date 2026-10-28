package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"voice/backend/file/internal/inventory"
	"voice/backend/file/internal/store"
)

var storyInventorySettings = []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "TLS_CERT_FILE", "TLS_KEY_FILE"}

const storyInventoryPrefix = "FILE_STORY_REFERENCE_INVENTORY_"

func loadStoryInventorySource(file *filePrincipalRuntime) (*inventory.StorySource, error) {
	settings := map[string]string{}
	enabled := false
	for _, name := range storyInventorySettings {
		v, set := os.LookupEnv(storyInventoryPrefix + name)
		enabled = enabled || set
		settings[name] = strings.TrimSpace(v)
	}
	if !enabled {
		return nil, nil
	}
	for _, name := range storyInventorySettings {
		if settings[name] == "" {
			return nil, errors.New("complete File Story inventory mTLS configuration required")
		}
	}
	if file == nil || file.Issuer == nil {
		return nil, errors.New("file principal runtime required for Story inventory")
	}
	cert, err := tls.LoadX509KeyPair(settings["TLS_CERT_FILE"], settings["TLS_KEY_FILE"])
	if err != nil {
		return nil, err
	}
	pem, err := os.ReadFile(settings["TLS_CA_FILE"])
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("story inventory CA contains no certificate")
	}
	return inventory.DialStorySource(settings["GRPC_ADDR"], &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: settings["TLS_SERVER_NAME"], Certificates: []tls.Certificate{cert}}, file.Issuer)
}

func startStoryInventoryWorker(ctx context.Context, files *store.FilesStore, source *inventory.StorySource, logger *slog.Logger) error {
	if source == nil {
		return nil
	}
	var present bool
	if err := files.Pool.QueryRow(ctx, `SELECT to_regclass('file_reference_inventory_imports') IS NOT NULL`).Scan(&present); err != nil {
		return err
	}
	if !present {
		return errors.New("file reference inventory migration16 required")
	}
	worker := &inventory.Importer{Files: files, Source: source}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := worker.RunPendingOnce(ctx, 4); err != nil && logger != nil {
					logger.Warn("Story reference inventory remains pending", slog.String("error", err.Error()))
				}
			}
		}
	}()
	return nil
}
