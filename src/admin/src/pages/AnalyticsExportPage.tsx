import { useState } from "react";
import { exportAnalytics } from "../api/analytics";

export function AnalyticsExportPage() {
  const [eventType, setEventType] = useState("");
  const [status, setStatus] = useState<string | null>(null);

  async function onExport(format: "csv" | "json") {
    setStatus("Exporting…");
    try {
      const blob = await exportAnalytics(format, eventType || undefined);
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
        <button type="button" onClick={() => onExport("csv")}>
          Download CSV
        </button>
        <button type="button" onClick={() => onExport("json")}>
          Download JSON
        </button>
      </div>
      {status ? <p>{status}</p> : null}
    </section>
  );
}
