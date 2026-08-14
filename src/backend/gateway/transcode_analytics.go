package main

import (
	"net/http"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
)

func analyticsTimeRange(r *http.Request) (*timestamppb.Timestamp, *timestamppb.Timestamp) {
	var from, to *timestamppb.Timestamp
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = timestamppb.New(t.UTC())
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = timestamppb.New(t.UTC())
		}
	}
	return from, to
}

func analyticsFilters(r *http.Request) map[string]string {
	filters := map[string]string{}
	if v := strings.TrimSpace(r.URL.Query().Get("event_type")); v != "" {
		filters["event_type"] = v
	}
	for key, values := range r.URL.Query() {
		if !strings.HasPrefix(key, "filter_") || len(values) == 0 {
			continue
		}
		name := strings.TrimPrefix(key, "filter_")
		if name == "" {
			continue
		}
		filters[name] = values[0]
	}
	if len(filters) == 0 {
		return nil
	}
	return filters
}

func (t *transcoder) serveAnalytics(w http.ResponseWriter, r *http.Request, rest string) bool {
	if t.clients.analytics == nil {
		return false
	}
	ctx := withGRPCMetadata(r.Context(), r)
	from, to := analyticsTimeRange(r)

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
