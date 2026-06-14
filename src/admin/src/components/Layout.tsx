import type { ReactNode } from "react";
import { NavLink } from "react-router-dom";

interface LayoutProps {
  children: ReactNode;
}

export function Layout({ children }: LayoutProps) {
  return (
    <div className="app-shell">
      <header className="app-header">
        <h1>Voice Moderation</h1>
        <nav className="app-nav" aria-label="Main">
          <NavLink
            to="/queue"
            className={({ isActive }) => (isActive ? "active" : "")}
          >
            Queue
          </NavLink>
          <NavLink
            to="/audit"
            className={({ isActive }) => (isActive ? "active" : "")}
          >
            Audit log
          </NavLink>
        </nav>
      </header>
      <main className="app-main">{children}</main>
    </div>
  );
}
