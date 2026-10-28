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
const messagingInventoryPrefix = "FILE_MESSAGING_REFERENCE_INVENTORY_"

func loadMessagingInventorySource(file *filePrincipalRuntime) (*inventory.MessagingSource, error) {
	settings := map[string]string{}
	enabled := false
	for _, name := range storyInventorySettings {
		v, set := os.LookupEnv(messagingInventoryPrefix + name)
		enabled = enabled || set
		settings[name] = strings.TrimSpace(v)
	}
	if !enabled {
		return nil, nil
	}
	for _, name := range storyInventorySettings {
		if settings[name] == "" {
			return nil, errors.New("complete File Messaging inventory mTLS configuration required")
		}
	}
	if file == nil || file.Issuer == nil {
		return nil, errors.New("file principal runtime required for Messaging inventory")
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
		return nil, errors.New("messaging inventory CA contains no certificate")
	}
	return inventory.DialMessagingSource(settings["GRPC_ADDR"], &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: settings["TLS_SERVER_NAME"], Certificates: []tls.Certificate{cert}}, file.Issuer)
}

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
	return startInventoryWorker(ctx, files, &inventory.OwnerRouter{Story: source}, logger)
}

func startInventoryWorker(ctx context.Context, files *store.FilesStore, router *inventory.OwnerRouter, logger *slog.Logger) error {
	if router == nil || (router.Story == nil && router.Messaging == nil) {
		return nil
	}
	var present bool
	if err := files.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='file_reference_inventory_imports' AND column_name='component')`).Scan(&present); err != nil {
		return err
	}
	if !present {
		return errors.New("file reference inventory migration17 required")
	}
	worker := &inventory.Importer{Files: files, Router: router}
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := worker.RunPendingOnce(ctx, 4); err != nil && logger != nil {
					logger.Warn("Reference inventory remains pending", slog.String("error", err.Error()))
				}
			}
		}
	}()
	return nil
}
