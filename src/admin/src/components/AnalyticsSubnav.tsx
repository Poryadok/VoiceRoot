import { NavLink } from "react-router-dom";

const DASHBOARD_LINKS = [
  { to: "/analytics/product", label: "Product" },
  { to: "/analytics/engagement", label: "Engagement" },
  { to: "/analytics/revenue", label: "Revenue" },
  { to: "/analytics/health", label: "Health" },
  { to: "/analytics/moderation", label: "Moderation" },
  { to: "/analytics/retention", label: "Retention" },
  { to: "/analytics/funnels", label: "Funnels" },
  { to: "/analytics/export", label: "Export" },
] as const;

export function AnalyticsSubnav() {
  return (
    <nav className="analytics-subnav" aria-label="Analytics">
      {DASHBOARD_LINKS.map(({ to, label }) => (
        <NavLink
          key={to}
          to={to}
          className={({ isActive }) => (isActive ? "active" : "")}
        >
          {label}
        </NavLink>
      ))}
    </nav>
  );
}
