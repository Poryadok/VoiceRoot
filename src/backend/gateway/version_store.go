package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type clientVersionRecord struct {
	Platform            string
	MinSupportedVersion string
	LatestVersion       string
	UpdateURL           string
	ReleaseNotes        string
	ShorebirdPatch      int
}

type versionStore interface {
	Get(ctx context.Context, platform string) (clientVersionRecord, error)
	Set(ctx context.Context, record clientVersionRecord) error
}

var errUnknownPlatform = errors.New("unknown_platform")

func recordFromConfig(platform string, cfg versionConfig) clientVersionRecord {
	return clientVersionRecord{
		Platform:            platform,
		MinSupportedVersion: cfg.MinSupportedVersion,
		LatestVersion:       cfg.LatestVersion,
		UpdateURL:           cfg.UpdateURL,
		ReleaseNotes:        cfg.ReleaseNotes,
		ShorebirdPatch:      cfg.ShorebirdPatch,
	}
}

type envVersionStore struct {
	configs map[string]versionConfig
}

func newEnvVersionStore(configs map[string]versionConfig) versionStore {
	if configs == nil {
		configs = map[string]versionConfig{}
	}
	return envVersionStore{configs: configs}
}

func (s envVersionStore) Get(_ context.Context, platform string) (clientVersionRecord, error) {
	cfg, ok := s.configs[platform]
	if !ok {
		return clientVersionRecord{}, fmt.Errorf("%w: %s", errUnknownPlatform, platform)
	}
	return recordFromConfig(platform, cfg), nil
}

func (s envVersionStore) Set(_ context.Context, record clientVersionRecord) error {
	s.configs[record.Platform] = versionConfig{
		MinSupportedVersion: record.MinSupportedVersion,
		LatestVersion:       record.LatestVersion,
		UpdateURL:           record.UpdateURL,
		ReleaseNotes:        record.ReleaseNotes,
		ShorebirdPatch:      record.ShorebirdPatch,
	}
	return nil
}

type postgresVersionStore struct {
	db *sql.DB
}

func newPostgresVersionStore(db *sql.DB) versionStore {
	return postgresVersionStore{db: db}
}

func (s postgresVersionStore) Get(ctx context.Context, platform string) (clientVersionRecord, error) {
	var rec clientVersionRecord
	var shorebird sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT platform, min_supported, latest_version, update_url, release_notes, shorebird_patch
		FROM client_versions
		WHERE platform = $1
	`, platform).Scan(
		&rec.Platform,
		&rec.MinSupportedVersion,
		&rec.LatestVersion,
		&rec.UpdateURL,
		&rec.ReleaseNotes,
		&shorebird,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return clientVersionRecord{}, fmt.Errorf("%w: %s", errUnknownPlatform, platform)
	}
	if err != nil {
		return clientVersionRecord{}, err
	}
	if shorebird.Valid {
		rec.ShorebirdPatch = int(shorebird.Int64)
	}
	return rec, nil
}

func (s postgresVersionStore) Set(ctx context.Context, record clientVersionRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO client_versions (
			platform, min_supported, latest_version, update_url, release_notes, shorebird_patch, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (platform) DO UPDATE SET
			min_supported = EXCLUDED.min_supported,
			latest_version = EXCLUDED.latest_version,
			update_url = EXCLUDED.update_url,
			release_notes = EXCLUDED.release_notes,
			shorebird_patch = EXCLUDED.shorebird_patch,
			updated_at = NOW()
	`,
		record.Platform,
		record.MinSupportedVersion,
		record.LatestVersion,
		record.UpdateURL,
		nullIfEmpty(record.ReleaseNotes),
		nullIfZero(record.ShorebirdPatch),
	)
	return err
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullIfZero(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

func versionStoreFromEnv(configs map[string]versionConfig, db *sql.DB) versionStore {
	if db != nil {
		return newPostgresVersionStore(db)
	}
	return newEnvVersionStore(configs)
}
