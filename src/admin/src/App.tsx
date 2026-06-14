import { Navigate, Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { AuditPage } from "./pages/AuditPage";
import { QueuePage } from "./pages/QueuePage";

export function App() {
  return (
    <Layout>
      <Routes>
        <Route path="/" element={<Navigate to="/queue" replace />} />
        <Route path="/queue" element={<QueuePage />} />
        <Route path="/audit" element={<AuditPage />} />
      </Routes>
    </Layout>
  );
}
