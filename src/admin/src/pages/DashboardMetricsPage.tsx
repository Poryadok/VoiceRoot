import { useEffect, useState } from "react";
import { fetchDashboard, type MetricPoint } from "../api/analytics";
import { AnalyticsSubnav } from "../components/AnalyticsSubnav";
import { DashboardMetricsTable } from "../components/DashboardMetricsTable";

const TITLES: Record<string, string> = {
  product: "Product analytics",
  engagement: "Engagement analytics",
  revenue: "Revenue analytics",
  health: "Health analytics",
  moderation: "Moderation analytics",
};

interface DashboardMetricsPageProps {
  dashboardType: keyof typeof TITLES;
}

export function DashboardMetricsPage({ dashboardType }: DashboardMetricsPageProps) {
  const [metrics, setMetrics] = useState<MetricPoint[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    setError(null);
    fetchDashboard(dashboardType)
      .then((r) => setMetrics(r.metrics ?? []))
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [dashboardType]);

  return (
    <>
      <AnalyticsSubnav />
      <DashboardMetricsTable
        title={TITLES[dashboardType]}
        metrics={metrics}
        error={error}
        loading={loading}
      />
    </>
  );
}
