import { apiJson, apiFetch } from "./client";
import type {
  ApplySanctionResponse,
  AuditExportResponse,
  ListReportsResponse,
  ModerationQueue,
  ResolveReportResponse,
  SanctionType,
} from "./types";

export interface ListReportsParams {
  status?: string;
  queue: ModerationQueue;
}

export function listReports(
  params: ListReportsParams,
): Promise<ListReportsResponse> {
  const search = new URLSearchParams();
  if (params.status) {
    search.set("status", params.status);
  }
  search.set("queue", params.queue);
  const query = search.toString();
  return apiJson(`/api/v1/admin/moderation/reports?${query}`);
}

export function applySanction(body: {
  target_account_id: string;
  type: SanctionType;
  reason: string;
  report_id?: string;
}): Promise<ApplySanctionResponse> {
  return apiJson("/api/v1/admin/moderation/sanctions", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function resolveReport(
  reportId: string,
  body: {
    new_status: string;
    resolution_json?: string;
    assigned_to_profile_id?: string;
  },
): Promise<ResolveReportResponse> {
  return apiJson(`/api/v1/admin/moderation/reports/${reportId}/resolve`, {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function fetchAuditExport(): Promise<AuditExportResponse> {
  return apiJson("/api/v1/admin/moderation/audit/export");
}

export async function downloadAuditExport(): Promise<void> {
  const response = await apiFetch("/api/v1/admin/moderation/audit/export");
  if (!response.ok) {
    throw new Error(await response.text());
  }
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `moderation-audit-${new Date().toISOString().slice(0, 10)}.json`;
  anchor.click();
  URL.revokeObjectURL(url);
}
