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

  it("offers documented genres and keeps custom metadata editable", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const config = {
      ...defaultGameConfig(),
      genre: "Puzzle",
      platforms: ["handheld"],
    };
    render(<GameConfigEditor config={config} onChange={onChange} />);

    const genre = screen.getByRole("combobox", { name: "Genre (optional)" });
    expect(genre).toHaveValue("Puzzle");
    const suggestions = document.getElementById(genre.getAttribute("list")!);
    expect(Array.from(suggestions!.querySelectorAll("option"), (option) => option.value))
      .toEqual(["Action", "MOBA", "Shooter", "Strategy"]);
    await user.click(screen.getByRole("checkbox", { name: "PC" }));
    expect(onChange).toHaveBeenLastCalledWith({
      ...config,
      platforms: ["handheld", "pc"],
    });
  });

  it("removes a selected platform without discarding custom platforms", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const config = {
      ...defaultGameConfig(),
      platforms: ["pc", "handheld", "console"],
    };
    render(<GameConfigEditor config={config} onChange={onChange} />);

    expect(screen.getByRole("checkbox", { name: "PC" })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: "Mobile" })).not.toBeChecked();
    await user.click(screen.getByRole("checkbox", { name: "PC" }));
    expect(onChange).toHaveBeenLastCalledWith({
      ...config,
      platforms: ["handheld", "console"],
    });
  });
});
