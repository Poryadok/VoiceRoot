package grpcsvc

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/analytics/internal/store"
)

// QueryGRPC implements AnalyticsQueryService.
type QueryGRPC struct {
	analyticsv1.UnimplementedAnalyticsQueryServiceServer
	Store *store.CHStore
}

func validatedRangeFromReq(from, to *timestamppb.Timestamp, now time.Time) (time.Time, time.Time, error) {
	now = now.UTC()
	end := now
	start := now.Add(-30 * 24 * time.Hour)
	if from != nil {
		if err := from.CheckValid(); err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid from timestamp: %w", err)
		}
		start = from.AsTime().UTC()
	}
	if to != nil {
		if err := to.CheckValid(); err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid to timestamp: %w", err)
		}
		end = to.AsTime().UTC()
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("from must be before to")
	}
	if end.After(now) {
		return time.Time{}, time.Time{}, fmt.Errorf("to must not be in the future")
	}
	return start, end, nil
}

func rangeFromReq(from, to *timestamppb.Timestamp) (time.Time, time.Time, error) {
	return validatedRangeFromReq(from, to, time.Now())
}

func filtersFromReq(filters map[string]string) store.QueryFilters {
	out := store.QueryFilters{}
	if filters == nil {
		return out
	}
	if v := strings.TrimSpace(filters["event_type"]); v != "" {
		out.EventType = v
	}
	return out
}

func (s *QueryGRPC) GetDashboard(ctx context.Context, req *analyticsv1.GetDashboardRequest) (*analyticsv1.GetDashboardResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "analytics store unavailable")
	}
	from, to, err := rangeFromReq(req.GetFrom(), req.GetTo())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	metrics, err := s.Store.DashboardMetrics(ctx, req.GetDashboardType(), from, to, store.QueryFilters{})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &analyticsv1.GetDashboardResponse{DashboardType: req.GetDashboardType()}
	for _, name := range sortedMetricNames(metrics) {
		value := metrics[name]
		n := name
		resp.Metrics = append(resp.Metrics, &analyticsv1.MetricPoint{Name: n, Value: value})
	}
	return resp, nil
}

func (s *QueryGRPC) GetMetrics(ctx context.Context, req *analyticsv1.GetMetricsRequest) (*analyticsv1.GetMetricsResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "analytics store unavailable")
	}
	from, to, err := rangeFromReq(req.GetFrom(), req.GetTo())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	dt := strings.TrimSpace(req.GetMetric())
	if dt == "" {
		return nil, status.Error(codes.InvalidArgument, "metric required")
	}
	m, err := s.Store.DashboardMetrics(ctx, dt, from, to, filtersFromReq(req.GetFilters()))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &analyticsv1.GetMetricsResponse{}
	for _, name := range sortedMetricNames(m) {
		value := m[name]
		n := name
		resp.Points = append(resp.Points, &analyticsv1.MetricPoint{Name: n, Value: value})
	}
	return resp, nil
}

func (s *QueryGRPC) GetFunnel(ctx context.Context, req *analyticsv1.GetFunnelRequest) (*analyticsv1.GetFunnelResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "analytics store unavailable")
	}
	from, to, err := rangeFromReq(req.GetFrom(), req.GetTo())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	steps, err := s.Store.FunnelSteps(ctx, req.GetFunnelName(), from, to)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &analyticsv1.GetFunnelResponse{FunnelName: req.GetFunnelName()}
	for step, count := range steps {
		st := step
		resp.Steps = append(resp.Steps, &analyticsv1.FunnelStep{Step: st, Count: count})
	}
	return resp, nil
}

func (s *QueryGRPC) GetRetention(ctx context.Context, req *analyticsv1.GetRetentionRequest) (*analyticsv1.GetRetentionResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "analytics store unavailable")
	}
	from, to, err := rangeFromReq(req.GetCohortFrom(), req.GetCohortTo())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	rows, err := s.Store.RetentionCohorts(ctx, from, to)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &analyticsv1.GetRetentionResponse{}
	for _, r := range rows {
		resp.Cohorts = append(resp.Cohorts, &analyticsv1.RetentionCohort{
			CohortDate: r.CohortDate,
			CohortSize: r.CohortSize,
			D1:         r.D1,
			D7:         r.D7,
			D30:        r.D30,
		})
	}
	return resp, nil
}

func (s *QueryGRPC) ExportData(ctx context.Context, req *analyticsv1.ExportDataRequest) (*analyticsv1.ExportDataResponse, error) {
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "analytics store unavailable")
	}
	from, to, err := rangeFromReq(req.GetFrom(), req.GetTo())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	rows, err := s.Store.ExportEvents(ctx, from, to, req.GetEventType(), 10000)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	format := strings.ToLower(strings.TrimSpace(req.GetFormat()))
	if format == "" {
		format = "csv"
	}
	switch format {
	case "json":
		type row struct {
			EventID       string    `json:"event_id"`
			EventType     string    `json:"event_type"`
			SourceService string    `json:"source_service"`
			Timestamp     time.Time `json:"timestamp"`
			Properties    string    `json:"properties"`
		}
		out := make([]row, 0, len(rows))
		for _, r := range rows {
			out = append(out, row{
				EventID: r.EventID, EventType: r.EventType, SourceService: r.SourceService,
				Timestamp: r.Timestamp, Properties: r.PropertiesJSON,
			})
		}
		b, err := json.Marshal(out)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return &analyticsv1.ExportDataResponse{ContentType: "application/json", Body: b}, nil
	case "csv":
		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		_ = w.Write([]string{"event_id", "event_type", "source_service", "timestamp", "user_id_hashed", "profile_id_hashed", "properties"})
		for _, r := range rows {
			_ = w.Write([]string{r.EventID, r.EventType, r.SourceService, r.Timestamp.Format(time.RFC3339Nano), r.UserIDHashed, r.ProfileIDHashed, r.PropertiesJSON})
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		return &analyticsv1.ExportDataResponse{ContentType: "text/csv", Body: buf.Bytes()}, nil
	default:
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("unsupported format %q", format))
	}
}

func sortedMetricNames(metrics map[string]float64) []string {
	names := make([]string, 0, len(metrics))
	for name := range metrics {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
