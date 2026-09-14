import { useState } from "react";
import { exportAnalytics } from "../api/analytics";
import { AnalyticsSubnav } from "../components/AnalyticsSubnav";
import {
  AnalyticsTimeRangeFilter,
  AnalyticsTimeRangeScope,
  useAnalyticsTimeRange,
} from "../components/AnalyticsTimeRange";

function AnalyticsExportContent() {
  const [eventType, setEventType] = useState("");
  const [status, setStatus] = useState<string | null>(null);
  const { range, isRangeValid } = useAnalyticsTimeRange();

  async function onExport(format: "csv" | "json") {
    if (!isRangeValid) {
      return;
    }
    setStatus("Exporting…");
    try {
      const blob = await exportAnalytics(format, eventType || undefined, range);
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `voice-analytics.${format}`;
      a.click();
      URL.revokeObjectURL(url);
      setStatus("Done");
    } catch (e) {
      setStatus(e instanceof Error ? e.message : "Export failed");
    }
  }

  return (
    <>
      <AnalyticsSubnav />
      <AnalyticsTimeRangeFilter />
      <section>
      <h2>Export analytics</h2>
      <label>
        Event type filter (optional)
        <input
          value={eventType}
          onChange={(e) => setEventType(e.target.value)}
          placeholder="message_sent"
        />
      </label>
      <div className="button-row">
        <button type="button" onClick={() => onExport("csv")} disabled={!isRangeValid}>
          Download CSV
        </button>
        <button type="button" onClick={() => onExport("json")} disabled={!isRangeValid}>
          Download JSON
        </button>
      </div>
      {status ? <p>{status}</p> : null}
    </section>
    </>
  );
}

export function AnalyticsExportPage() {
  return <AnalyticsTimeRangeScope><AnalyticsExportContent /></AnalyticsTimeRangeScope>;
}
