import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { App } from "../App";
import * as moderation from "../api/moderation";
import * as gameCatalog from "../api/gameCatalog";

vi.mock("../api/moderation", () => ({
  listReports: vi.fn(),
  resolveReport: vi.fn(),
  applySanction: vi.fn(),
  fetchAccountSanctions: vi.fn(),
  revokeSanction: vi.fn(),
}));

vi.mock("../api/gameCatalog", () => ({
  searchGames: vi.fn(),
  createGame: vi.fn(),
  findDuplicateName: vi.fn(),
}));

function staffToken(profileId = "staff-profile"): string {
  const payload = btoa(JSON.stringify({ profile_id: profileId }));
  return `hdr.${payload}.sig`;
}

describe("App shell routing", () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.stubEnv("VITE_OAUTH_DISABLED", "true");
    vi.stubEnv("VITE_STAFF_TOKEN", "");
    sessionStorage.setItem("voice_admin_access_token", staffToken());

    vi.mocked(moderation.listReports).mockResolvedValue({
      report_list: { reports: [] },
    });
    vi.mocked(moderation.fetchAccountSanctions).mockResolvedValue({
      sanction_list: { sanctions: [] },
    });
    vi.mocked(gameCatalog.searchGames).mockResolvedValue({ games: [] });
    vi.mocked(gameCatalog.findDuplicateName).mockReturnValue(undefined);
    vi.mocked(gameCatalog.createGame).mockResolvedValue({
      game: { id: "g-new", name: "New Game", status: "active" },
    });
  });

  it("redirects / to /queue and renders moderation shell", async () => {
    render(
      <MemoryRouter initialEntries={["/"]}>
        <App />
      </MemoryRouter>,
    );

    expect(await screen.findByRole("navigation", { name: "Main" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Moderation" })).toHaveClass("active");
  });

  it("renders add-game page at /games/new", async () => {
    render(
      <MemoryRouter initialEntries={["/games/new"]}>
        <App />
      </MemoryRouter>,
    );

    expect(await screen.findByRole("heading", { name: "Add game to catalog" })).toBeInTheDocument();
    expect(screen.getByText("Matchmaking configuration")).toBeInTheDocument();
  });

  it("renders OAuth callback route outside admin shell", () => {
    render(
      <MemoryRouter initialEntries={["/callback"]}>
        <App />
      </MemoryRouter>,
    );

    expect(screen.queryByRole("navigation", { name: "Main" })).not.toBeInTheDocument();
  });
});
