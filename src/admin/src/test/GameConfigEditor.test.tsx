import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { GameConfigEditor } from "../components/GameConfigEditor";
import { defaultGameConfig } from "../lib/gameConfig";

describe("GameConfigEditor", () => {
  it("adds a role row for the active mode", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <GameConfigEditor
        config={defaultGameConfig()}
        onChange={onChange}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Add role" }));

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({
        modes: [
          expect.objectContaining({
            roles: [{ name: "", required: false }],
          }),
        ],
      }),
    );
  });

  it("shows validation error from parent", () => {
    render(
      <GameConfigEditor
        config={defaultGameConfig()}
        onChange={() => undefined}
        validationError={'Mode 1: duplicate role "Carry".'}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent("duplicate role");
  });
});
