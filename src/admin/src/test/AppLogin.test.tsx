import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { App } from "../App";

describe("App login screen", () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.stubEnv("VITE_OAUTH_DISABLED", "true");
    vi.stubEnv("VITE_STAFF_TOKEN", "");
  });

  it("shows staff JWT form when oauth is disabled", () => {
    render(
      <MemoryRouter initialEntries={["/queue"]}>
        <App />
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: "Voice Admin" })).toBeInTheDocument();
    expect(screen.getByLabelText("Staff JWT")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Continue" })).toBeInTheDocument();
  });

  it("stores pasted JWT and reloads on continue", async () => {
    const user = userEvent.setup();
    const reloadSpy = vi.fn();
    Object.defineProperty(window, "location", {
      configurable: true,
      value: { ...window.location, reload: reloadSpy },
    });

    render(
      <MemoryRouter initialEntries={["/queue"]}>
        <App />
      </MemoryRouter>,
    );

    await user.type(screen.getByLabelText("Staff JWT"), " pasted-staff-jwt ");
    await user.click(screen.getByRole("button", { name: "Continue" }));

    expect(sessionStorage.getItem("voice_admin_access_token")).toBe(
      "pasted-staff-jwt",
    );
    expect(reloadSpy).toHaveBeenCalled();
  });

  it("shows status when continue clicked with empty JWT", async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter initialEntries={["/queue"]}>
        <App />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole("button", { name: "Continue" }));

    expect(screen.getByRole("status")).toHaveTextContent("Paste a staff JWT first.");
  });

  it("shows OAuth sign-in button when oauth enabled", () => {
    vi.stubEnv("VITE_OAUTH_DISABLED", "false");

    render(
      <MemoryRouter initialEntries={["/queue"]}>
        <App />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole("button", { name: "Sign in with Voice" }),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Staff JWT")).not.toBeInTheDocument();
  });
});
