package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	analyticsv1 "voice.app/voice/analytics/v1"
)

// EventRow is a denormalized analytics event for ClickHouse insert.
type EventRow struct {
	EventID         string
	EventType       string
	SourceService   string
	Timestamp       time.Time
	UserIDHashed    string
	ProfileIDHashed string
	PropertiesJSON  string
	SessionID       string
	Platform        string
	AppVersion      string
	Region          string
}

// CHStore reads and writes analytics events in ClickHouse.
type CHStore struct {
	conn driver.Conn
}

func Open(ctx context.Context, dsn string) (*CHStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("empty clickhouse dsn")
	}
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("clickhouse ping: %w", err)
	}
	return &CHStore{conn: conn}, nil
}

func (s *CHStore) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

func (s *CHStore) InsertBatch(ctx context.Context, rows []EventRow) error {
	if s == nil || s.conn == nil || len(rows) == 0 {
		return nil
	}
	batch, err := s.conn.PrepareBatch(ctx, `INSERT INTO voice.events (
		event_id, event_type, source_service, timestamp,
		user_id_hashed, profile_id_hashed, properties,
		session_id, platform, app_version, region
	)`)
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := batch.Append(
			r.EventID, r.EventType, r.SourceService, r.Timestamp,
			r.UserIDHashed, r.ProfileIDHashed, r.PropertiesJSON,
			nullable(r.SessionID), nullable(r.Platform), nullable(r.AppVersion), nullable(r.Region),
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func nullable(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func RowFromProto(ev *analyticsv1.AnalyticsEvent) EventRow {
	if ev == nil {
		return EventRow{}
	}
	ts := time.Now().UTC()
	if ev.GetTimestamp() != nil {
		ts = ev.GetTimestamp().AsTime().UTC()
	}
	return EventRow{
		EventID:         ev.GetEventId(),
		EventType:       ev.GetEventType(),
		SourceService:   ev.GetSourceService(),
		Timestamp:       ts,
		UserIDHashed:    ev.GetUserIdHashed(),
		ProfileIDHashed: ev.GetProfileIdHashed(),
		PropertiesJSON:  ev.GetPropertiesJson(),
		SessionID:       ev.GetSessionId(),
		Platform:        ev.GetPlatform(),
		AppVersion:      ev.GetAppVersion(),
		Region:          ev.GetRegion(),
	}
}
