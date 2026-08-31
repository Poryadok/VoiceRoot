import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { CreateGamePage } from "../pages/CreateGamePage";
import * as api from "../api/gameCatalog";

vi.mock("../api/gameCatalog", () => ({
  searchGames: vi.fn(),
  createGame: vi.fn(),
  findDuplicateName: vi.fn(),
}));

describe("CreateGamePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(api.searchGames).mockResolvedValue({ games: [] });
    vi.mocked(api.createGame).mockResolvedValue({
      game: { id: "g-new", name: "New Game", status: "active" },
    });
    vi.mocked(api.findDuplicateName).mockReturnValue(undefined);
  });

  it("creates a game when name is unique", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <CreateGamePage />
      </MemoryRouter>,
    );

    await user.type(screen.getByLabelText("Name"), "New Game");
    await waitFor(() => {
      expect(api.searchGames).toHaveBeenCalled();
    });
    await user.click(screen.getByRole("button", { name: "Create game" }));

    await waitFor(() => {
      expect(api.createGame).toHaveBeenCalledWith(
        expect.objectContaining({ name: "New Game" }),
      );
    });
    expect(await screen.findByRole("status")).toHaveTextContent("Created game");
  });

  it("blocks submit when duplicate detected", async () => {
    vi.mocked(api.findDuplicateName).mockImplementation((name, games) =>
      games?.find((g) => g.name.trim().toLowerCase() === name.trim().toLowerCase()),
    );
    vi.mocked(api.searchGames).mockResolvedValue({
      games: [{ id: "g1", name: "Apex Legends", status: "active" }],
    });

    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <CreateGamePage />
      </MemoryRouter>,
    );

    await user.type(screen.getByLabelText("Name"), "Apex Legends");
    expect(await screen.findByRole("alert")).toHaveTextContent("Duplicate");
    expect(screen.getByRole("button", { name: "Create game" })).toBeDisabled();
    expect(api.createGame).not.toHaveBeenCalled();
  });
});
