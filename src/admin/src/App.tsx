import { Navigate, Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { AuditPage } from "./pages/AuditPage";
import { AnalyticsExportPage } from "./pages/AnalyticsExportPage";
import { FunnelsPage } from "./pages/FunnelsPage";
import { ProductAnalyticsPage } from "./pages/ProductAnalyticsPage";
import { QueuePage } from "./pages/QueuePage";

export function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/queue" replace />} />
        <Route path="/queue" element={<QueuePage />} />
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/analytics/product" element={<ProductAnalyticsPage />} />
        <Route path="/analytics/funnels" element={<FunnelsPage />} />
        <Route path="/analytics/export" element={<AnalyticsExportPage />} />
      </Routes>
    </Layout>
  );
}
