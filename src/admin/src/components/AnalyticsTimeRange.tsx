import { createContext, useContext, useMemo, useState, type ReactNode } from "react";
import type { AnalyticsTimeRange } from "../api/analytics";

interface AnalyticsTimeRangeState {
  from: string;
  to: string;
  setFrom: (value: string) => void;
  setTo: (value: string) => void;
  range?: AnalyticsTimeRange;
  isRangeValid: boolean;
  validationError?: string;
}

const AnalyticsTimeRangeContext = createContext<AnalyticsTimeRangeState | null>(null);

function utcTimestamp(value: string): string | undefined {
  if (!value) {
    return undefined;
  }

  const timestamp = new Date(`${value.length === 16 ? `${value}:00` : value}Z`);
  return Number.isNaN(timestamp.getTime())
    ? undefined
    : timestamp.toISOString().replace(".000Z", "Z");
}

function useTimeRangeState(): AnalyticsTimeRangeState {
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const { range, isRangeValid, validationError } = useMemo(() => {
    const utcFrom = utcTimestamp(from);
    const utcTo = utcTimestamp(to);
    const isRangeValid = !utcFrom || !utcTo || utcFrom <= utcTo;
    return {
      range: isRangeValid && (utcFrom || utcTo) ? { from: utcFrom, to: utcTo } : undefined,
      isRangeValid,
      validationError: isRangeValid ? undefined : "From must be before or equal to To.",
    };
  }, [from, to]);

  return { from, to, setFrom, setTo, range, isRangeValid, validationError };
}

export function AnalyticsTimeRangeProvider({ children }: { children: ReactNode }) {
  const value = useTimeRangeState();
  return <AnalyticsTimeRangeContext.Provider value={value}>{children}</AnalyticsTimeRangeContext.Provider>;
}

export function AnalyticsTimeRangeScope({ children }: { children: ReactNode }) {
  const sharedRange = useContext(AnalyticsTimeRangeContext);
  return sharedRange ? <>{children}</> : <AnalyticsTimeRangeProvider>{children}</AnalyticsTimeRangeProvider>;
}

export function useAnalyticsTimeRange(): AnalyticsTimeRangeState {
  const sharedRange = useContext(AnalyticsTimeRangeContext);
  const localRange = useTimeRangeState();
  return sharedRange ?? localRange;
}

export function AnalyticsTimeRangeFilter() {
  const { from, to, setFrom, setTo, validationError } = useAnalyticsTimeRange();

  return (
    <div className="filters" aria-label="Analytics time range">
      <label>
        From
        <input
          type="datetime-local"
          value={from}
          aria-invalid={Boolean(validationError)}
          aria-describedby={validationError ? "analytics-time-range-error" : undefined}
          onChange={(event) => setFrom(event.target.value)}
        />
      </label>
      <label>
        To
        <input
          type="datetime-local"
          value={to}
          aria-invalid={Boolean(validationError)}
          aria-describedby={validationError ? "analytics-time-range-error" : undefined}
          onChange={(event) => setTo(event.target.value)}
        />
      </label>
      <span>UTC</span>
      {validationError ? <p id="analytics-time-range-error" role="alert">{validationError}</p> : null}
    </div>
  );
}
