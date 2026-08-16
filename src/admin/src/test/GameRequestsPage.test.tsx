import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { GameRequestsPage } from "../pages/GameRequestsPage";
import * as api from "../api/gameRequests";

vi.mock("../api/gameRequests", () => ({
  listGameRequests: vi.fn(),
  approveGameRequest: vi.fn(),
  rejectGameRequest: vi.fn(),
}));

describe("GameRequestsPage", () => {
  beforeEach(() => {
    vi.mocked(api.listGameRequests).mockResolvedValue({
      game_list: {
        games: [{ id: "g1", name: "Apex Legends", status: "pending_moderation" }],
      },
    });
    vi.mocked(api.approveGameRequest).mockResolvedValue({
      game: { id: "g1", name: "Apex Legends", status: "active" },
    });
  });

  it("lists pending requests and approves", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <GameRequestsPage />
      </MemoryRouter>,
    );

    expect(await screen.findByText("Apex Legends")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Approve" }));
    await waitFor(() => {
      expect(api.approveGameRequest).toHaveBeenCalledWith("g1");
    });
  });
});
