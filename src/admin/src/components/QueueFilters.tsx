import type { ReportCategory, ReportStatus } from "../api/types";
import { REPORT_CATEGORIES, REPORT_STATUSES } from "../api/types";

export interface QueueFiltersValue {
  status: ReportStatus | "";
  category: ReportCategory | "";
}

interface QueueFiltersProps {
  value: QueueFiltersValue;
  onChange: (next: QueueFiltersValue) => void;
}

export function QueueFilters({ value, onChange }: QueueFiltersProps) {
  return (
    <div className="filters" data-testid="queue-filters">
      <label>
        Status
        <select
          data-testid="filter-status"
          value={value.status}
          onChange={(event) =>
            onChange({
              ...value,
              status: event.target.value as ReportStatus | "",
            })
          }
        >
          <option value="">All statuses</option>
          {REPORT_STATUSES.map((status) => (
            <option key={status} value={status}>
              {status}
            </option>
          ))}
        </select>
      </label>
      <label>
        Category
        <select
          data-testid="filter-category"
          value={value.category}
          onChange={(event) =>
            onChange({
              ...value,
              category: event.target.value as ReportCategory | "",
            })
          }
        >
          <option value="">All categories</option>
          {REPORT_CATEGORIES.map((category) => (
            <option key={category} value={category}>
              {category}
            </option>
          ))}
        </select>
      </label>
    </div>
  );
}

export function filterReportsByCategory<
  T extends { category: string },
>(reports: T[], category: ReportCategory | ""): T[] {
  if (!category) {
    return reports;
  }
  return reports.filter((report) => report.category === category);
}
