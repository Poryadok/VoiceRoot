package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
)

func parseAnalyticsTimeRange(r *http.Request, now time.Time) (*timestamppb.Timestamp, *timestamppb.Timestamp, error) {
	parse := func(name string) (*time.Time, error) {
		value := strings.TrimSpace(r.URL.Query().Get(name))
		if value == "" {
			return nil, nil
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, fmt.Errorf("invalid %s timestamp", name)
		}
		parsed = parsed.UTC()
		return &parsed, nil
	}
	from, err := parse("from")
	if err != nil {
		return nil, nil, err
	}
	to, err := parse("to")
	if err != nil {
		return nil, nil, err
	}
	now = now.UTC()
	start := now.Add(-30 * 24 * time.Hour)
	end := now
	if from != nil {
		start = *from
	}
	if to != nil {
		end = *to
	}
	if !start.Before(end) {
		return nil, nil, fmt.Errorf("from must be before to")
	}
	if end.After(now) {
		return nil, nil, fmt.Errorf("to must not be in the future")
	}
	return timestamppb.New(start), timestamppb.New(end), nil
}

func analyticsFilters(r *http.Request) map[string]string {
	if v := strings.TrimSpace(r.URL.Query().Get("event_type")); v != "" {
		return map[string]string{"event_type": v}
	}
	return nil
}

func (t *transcoder) serveAnalytics(w http.ResponseWriter, r *http.Request, rest string) bool {
	if t.clients.analytics == nil {
		return false
	}
	ctx := withGRPCMetadata(r.Context(), r)
	from, to, rangeErr := parseAnalyticsTimeRange(r, time.Now())
	if rangeErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_argument"})
		return true
	}

	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(rest, "dashboard/"):
		dashboardType := strings.TrimPrefix(rest, "dashboard/")
		resp, err := t.clients.analytics.GetDashboard(ctx, &analyticsv1.GetDashboardRequest{
			DashboardType: dashboardType,
			From:          from,
			To:            to,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && strings.HasPrefix(rest, "funnel/"):
		name := strings.TrimPrefix(rest, "funnel/")
		resp, err := t.clients.analytics.GetFunnel(ctx, &analyticsv1.GetFunnelRequest{
			FunnelName: name,
			From:       from,
			To:         to,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "retention":
		resp, err := t.clients.analytics.GetRetention(ctx, &analyticsv1.GetRetentionRequest{
			CohortFrom: from,
			CohortTo:   to,
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "metrics":
		metric := strings.TrimSpace(r.URL.Query().Get("metric"))
		resp, err := t.clients.analytics.GetMetrics(ctx, &analyticsv1.GetMetricsRequest{
			Metric:  metric,
			From:    from,
			To:      to,
			Filters: analyticsFilters(r),
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "export":
		format := strings.TrimSpace(r.URL.Query().Get("format"))
		if format == "" {
			format = "csv"
		}
		eventType := strings.TrimSpace(r.URL.Query().Get("event_type"))
		req := &analyticsv1.ExportDataRequest{
			Format: format,
			From:   from,
			To:     to,
		}
		if eventType != "" {
			req.EventType = &eventType
		}
		resp, err := t.clients.analytics.ExportData(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		w.Header().Set("Content-Type", resp.GetContentType())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(resp.GetBody())
		return true

	default:
		return false
	}
}
