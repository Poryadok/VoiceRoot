import type { ReactNode } from "react";
import { NavLink } from "react-router-dom";

interface LayoutProps {
  children: ReactNode;
  onSignOut?: () => void;
}

export function Layout({ children, onSignOut }: LayoutProps) {
  return (
    <div className="app-shell">
      <header className="app-header">
        <h1>Voice Admin</h1>
        <nav className="app-nav" aria-label="Main">
          <NavLink
            to="/queue"
            className={({ isActive }) => (isActive ? "active" : "")}
          >
            Moderation
          </NavLink>
          <NavLink
            to="/analytics/product"
            className={({ isActive }) => (isActive ? "active" : "")}
          >
            Analytics
          </NavLink>
          <NavLink
            to="/analytics/funnels"
            className={({ isActive }) => (isActive ? "active" : "")}
          >
            Funnels
          </NavLink>
          <NavLink
            to="/analytics/export"
            className={({ isActive }) => (isActive ? "active" : "")}
          >
            Export
          </NavLink>
          <NavLink
            to="/audit"
            className={({ isActive }) => (isActive ? "active" : "")}
          >
            Audit log
          </NavLink>
          {onSignOut ? (
            <button type="button" className="nav-signout" onClick={onSignOut}>
              Sign out
            </button>
          ) : null}
        </nav>
      </header>
      <main className="app-main">{children}</main>
    </div>
  );
}
