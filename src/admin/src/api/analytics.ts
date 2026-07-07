import { apiGet, apiGetBlob } from "./client";

export interface MetricPoint {
  name: string;
  value: number;
  label?: string;
}

export interface DashboardResponse {
  dashboard_type: string;
  metrics: MetricPoint[];
}

export interface FunnelStep {
  step: string;
  count: number;
}

export interface FunnelResponse {
  funnel_name: string;
  steps: FunnelStep[];
}

export interface RetentionCohort {
  cohort_date: string;
  cohort_size: number;
  d1: number;
  d7: number;
  d30: number;
}

export interface RetentionResponse {
  cohorts: RetentionCohort[];
}

export async function fetchDashboard(type: string): Promise<DashboardResponse> {
  return apiGet<DashboardResponse>(`/api/v1/analytics/dashboard/${type}`);
}

export async function fetchFunnel(name: string): Promise<FunnelResponse> {
  return apiGet<FunnelResponse>(`/api/v1/analytics/funnel/${name}`);
}

export async function fetchRetention(): Promise<RetentionResponse> {
  return apiGet<RetentionResponse>("/api/v1/analytics/retention");
}

export async function exportAnalytics(format: string, eventType?: string): Promise<Blob> {
  const params = new URLSearchParams({ format });
  if (eventType) {
    params.set("event_type", eventType);
  }
  return apiGetBlob(`/api/v1/analytics/export?${params.toString()}`);
}
