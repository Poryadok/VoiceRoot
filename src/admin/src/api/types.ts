export type ReportStatus =
  | "pending"
  | "reviewing"
  | "resolved"
  | "dismissed";

export type ReportCategory =
  | "spam"
  | "harassment"
  | "offensive"
  | "fake"
  | "cheating"
  | "other";

export type ModerationQueue = "content" | "spaces";

export type SanctionType =
  | "warning"
  | "temp_ban"
  | "perm_ban"
  | "shadow_ban"
  | "mm_ban";

export interface Report {
  id: string;
  reporter_profile_id: string;
  target_type: string;
  target_id: string;
  category: string;
  description?: string;
  evidence_json: string;
  status: string;
  assigned_to_profile_id?: string;
  resolved_at?: string;
  resolution_json: string;
  created_at: string;
}

export interface ReportList {
  reports: Report[];
  next_cursor?: string;
}

export interface ListReportsResponse {
  report_list: ReportList;
}

export interface Sanction {
  id: string;
  target_account_id: string;
  type: string;
  reason: string;
  report_id?: string;
  issued_by_profile_id: string;
  expires_at?: string;
  revoked_at?: string;
  created_at: string;
}

export interface ApplySanctionResponse {
  sanction: Sanction;
}

export interface ResolveReportResponse {
  report: Report;
}

export interface AuditEntry {
  id?: string;
  actor_profile_id?: string;
  action?: string;
  target_type?: string;
  target_id?: string;
  details?: string;
  created_at?: string;
}

export interface AuditExportResponse {
  entries: AuditEntry[];
}

export const REPORT_STATUSES: ReportStatus[] = [
  "pending",
  "reviewing",
  "resolved",
  "dismissed",
];

export const REPORT_CATEGORIES: ReportCategory[] = [
  "spam",
  "harassment",
  "offensive",
  "fake",
  "cheating",
  "other",
];

export const SANCTION_TYPES: SanctionType[] = [
  "warning",
  "temp_ban",
  "perm_ban",
  "shadow_ban",
  "mm_ban",
];

export const DESTRUCTIVE_SANCTIONS = new Set<SanctionType>([
  "temp_ban",
  "perm_ban",
  "shadow_ban",
  "mm_ban",
]);
