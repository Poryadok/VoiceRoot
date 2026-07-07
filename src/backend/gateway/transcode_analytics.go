package main

import (
	"net/http"
	"strings"

	analyticsv1 "voice.app/voice/analytics/v1"
)

func (t *transcoder) serveAnalytics(w http.ResponseWriter, r *http.Request, rest string) bool {
	if t.clients.analytics == nil {
		return false
	}
	ctx := withGRPCMetadata(r.Context(), r)

	switch {
	case r.Method == http.MethodGet && strings.HasPrefix(rest, "dashboard/"):
		dashboardType := strings.TrimPrefix(rest, "dashboard/")
		resp, err := t.clients.analytics.GetDashboard(ctx, &analyticsv1.GetDashboardRequest{
			DashboardType: dashboardType,
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
		})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "retention":
		resp, err := t.clients.analytics.GetRetention(ctx, &analyticsv1.GetRetentionRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return true
		}
		writeProtoJSON(w, http.StatusOK, resp)
		return true

	case r.Method == http.MethodGet && rest == "metrics":
		metric := strings.TrimSpace(r.URL.Query().Get("metric"))
		resp, err := t.clients.analytics.GetMetrics(ctx, &analyticsv1.GetMetricsRequest{
			Metric: metric,
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
		resp, err := t.clients.analytics.ExportData(ctx, &analyticsv1.ExportDataRequest{
			Format:    format,
			EventType: eventType,
		})
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
