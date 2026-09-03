package grpcsvc

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/analytics/internal/store"
)

func TestFiltersFromReqEventType(t *testing.T) {
	got := filtersFromReq(map[string]string{"event_type": "api_request"})
	require.Equal(t, "api_request", got.EventType)
}

func TestFiltersFromReqEmpty(t *testing.T) {
	got := filtersFromReq(nil)
	require.Empty(t, got.EventType)
}

func TestExportDataCSVHeaderIncludesOnlyHashedIdentities(t *testing.T) {
	response := exportCSV(t, []store.EventRow{{
		EventID:         "event-1",
		EventType:       "message_sent",
		SourceService:   "messaging",
		Timestamp:       time.Date(2026, time.September, 3, 10, 11, 12, 0, time.UTC),
		UserIDHashed:    "account-hmac",
		ProfileIDHashed: "profile-hmac",
		PropertiesJSON:  `{}`,
	}})
	records := parseCSV(t, response.GetBody())

	expectedHeader := []string{
		"event_id",
		"event_type",
		"source_service",
		"timestamp",
		"user_id_hashed",
		"profile_id_hashed",
		"properties",
	}
	assert.Equal(t, expectedHeader, records[0])
	for _, rawIdentityColumn := range []string{
		"account_id",
		"user_id",
		"profile_id",
	} {
		require.NotContains(t, records[0], rawIdentityColumn)
	}
}

func TestExportDataCSVWritesHashedIdentityValuesExactly(t *testing.T) {
	timestamp := time.Date(2026, time.September, 3, 10, 11, 12, 123456789, time.UTC)
	response := exportCSV(t, []store.EventRow{{
		EventID:         "event-2",
		EventType:       "profile_created",
		SourceService:   "user",
		Timestamp:       timestamp,
		UserIDHashed:    "hmac-sha256:account/AbCdEf==",
		ProfileIDHashed: "hmac-sha256:profile/XyZ123==",
		PropertiesJSON:  `{"source":"onboarding"}`,
	}})
	records := parseCSV(t, response.GetBody())

	require.Len(t, records, 2)
	require.Equal(t, []string{
		"event-2",
		"profile_created",
		"user",
		timestamp.Format(time.RFC3339Nano),
		"hmac-sha256:account/AbCdEf==",
		"hmac-sha256:profile/XyZ123==",
		`{"source":"onboarding"}`,
	}, records[1])
}

func TestExportDataCSVKeepsEmptyHashesAlignedWithEscapedProperties(t *testing.T) {
	timestamp := time.Date(2026, time.September, 3, 12, 13, 14, 0, time.UTC)
	properties := "{\n  \"note\": \"comma, quote \\\"value\\\"\"\n}"
	response := exportCSV(t, []store.EventRow{{
		EventID:        "event-3",
		EventType:      "search_query",
		SourceService:  "search",
		Timestamp:      timestamp,
		PropertiesJSON: properties,
	}})
	records := parseCSV(t, response.GetBody())

	require.Len(t, records, 2)
	require.Len(t, records[0], 7)
	require.Len(t, records[1], 7)
	require.Equal(t, "", records[1][4], "missing user hash must remain an empty aligned cell")
	require.Equal(t, "", records[1][5], "missing profile hash must remain an empty aligned cell")
	require.Equal(t, properties, records[1][6], "CSV parsing must recover properties containing commas, quotes, and newlines")
}

func TestExportDataJSONPreservesExistingContractWithoutHashedIdentities(t *testing.T) {
	timestamp := time.Date(2026, time.September, 3, 15, 16, 17, 123456789, time.UTC)
	properties := "{\n  \"note\": \"quote \\\"value\\\" and newline\\n\"\n}"
	response := exportJSON(t, []store.EventRow{{
		EventID:         "event-json",
		EventType:       "message_sent",
		SourceService:   "messaging",
		Timestamp:       timestamp,
		UserIDHashed:    "hmac-sha256:account/AbCdEf==",
		ProfileIDHashed: "hmac-sha256:profile/XyZ123==",
		PropertiesJSON:  properties,
	}})

	var records []map[string]string
	require.NoError(t, json.Unmarshal(response.GetBody(), &records))
	require.Equal(t, []map[string]string{{
		"event_id":       "event-json",
		"event_type":     "message_sent",
		"source_service": "messaging",
		"timestamp":      timestamp.Format(time.RFC3339Nano),
		"properties":     properties,
	}}, records)
	require.NotContains(t, records[0], "user_id_hashed")
	require.NotContains(t, records[0], "profile_id_hashed")
}

func exportCSV(t *testing.T, rows []store.EventRow) *analyticsv1.ExportDataResponse {
	t.Helper()
	response := exportData(t, rows, "csv")
	require.Equal(t, "text/csv", response.GetContentType())
	return response
}

func exportJSON(t *testing.T, rows []store.EventRow) *analyticsv1.ExportDataResponse {
	t.Helper()
	response := exportData(t, rows, "json")
	require.Equal(t, "application/json", response.GetContentType())
	return response
}

func exportData(t *testing.T, rows []store.EventRow, format string) *analyticsv1.ExportDataResponse {
	t.Helper()
	queryStore := &store.CHStore{}
	setStoreConnection(t, queryStore, &exportConn{rows: rows})
	service := &QueryGRPC{Store: queryStore}

	response, err := service.ExportData(context.Background(), &analyticsv1.ExportDataRequest{
		From:   timestamppb.New(time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)),
		To:     timestamppb.New(time.Date(2026, time.September, 4, 0, 0, 0, 0, time.UTC)),
		Format: format,
	})
	require.NoError(t, err)
	return response
}

func parseCSV(t *testing.T, body []byte) [][]string {
	t.Helper()
	reader := csv.NewReader(strings.NewReader(string(body)))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, records)
	return records
}

func setStoreConnection(t *testing.T, queryStore *store.CHStore, connection driver.Conn) {
	t.Helper()
	connectionField := reflect.ValueOf(queryStore).Elem().FieldByName("conn")
	require.True(t, connectionField.IsValid(), "CHStore connection field must exist")
	writableField := reflect.NewAt(connectionField.Type(), unsafe.Pointer(connectionField.UnsafeAddr())).Elem()
	writableField.Set(reflect.ValueOf(connection))
}

type exportConn struct {
	driver.Conn
	rows []store.EventRow
}

func (c *exportConn) Query(_ context.Context, query string, _ ...any) (driver.Rows, error) {
	projection, _, found := strings.Cut(strings.ToLower(query), "from")
	if !found {
		return nil, fmt.Errorf("export query has no SELECT projection")
	}
	for _, column := range []string{"user_id_hashed", "profile_id_hashed"} {
		if !strings.Contains(projection, column) {
			return nil, fmt.Errorf("export query does not select %s", column)
		}
	}
	return &exportRows{position: -1, rows: c.rows}, nil
}

type exportRows struct {
	driver.Rows
	position int
	rows     []store.EventRow
}

func (r *exportRows) Next() bool {
	r.position++
	return r.position < len(r.rows)
}

func (r *exportRows) Scan(dest ...any) error {
	if r.position < 0 || r.position >= len(r.rows) {
		return fmt.Errorf("scan called outside current row")
	}
	if len(dest) != 11 {
		return fmt.Errorf("scan received %d destinations, want 11", len(dest))
	}
	row := r.rows[r.position]
	values := []string{
		row.EventID,
		row.EventType,
		row.SourceService,
		row.UserIDHashed,
		row.ProfileIDHashed,
		row.PropertiesJSON,
		row.SessionID,
		row.Platform,
		row.AppVersion,
		row.Region,
	}
	for index, value := range values[:3] {
		field, ok := dest[index].(*string)
		if !ok {
			return fmt.Errorf("destination %d has type %T, want *string", index, dest[index])
		}
		*field = value
	}
	timestamp, ok := dest[3].(*time.Time)
	if !ok {
		return fmt.Errorf("destination 3 has type %T, want *time.Time", dest[3])
	}
	*timestamp = row.Timestamp
	for index, value := range values[3:] {
		destinationIndex := index + 4
		field, ok := dest[destinationIndex].(*string)
		if !ok {
			return fmt.Errorf("destination %d has type %T, want *string", destinationIndex, dest[destinationIndex])
		}
		*field = value
	}
	return nil
}

func (r *exportRows) Close() error { return nil }

func (r *exportRows) Err() error { return nil }
