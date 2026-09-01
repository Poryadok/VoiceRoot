import type { GameConfig, GameMode, GameRank, GameRole } from "../lib/gameConfig";
import {
  formatListInput,
  parseListInput,
} from "../lib/gameConfig";

interface GameConfigEditorProps {
  config: GameConfig;
  onChange: (next: GameConfig) => void;
  validationError?: string;
}

function updateMode(
  config: GameConfig,
  index: number,
  patch: Partial<GameMode>,
): GameConfig {
  const modes = config.modes.map((mode, i) =>
    i === index ? { ...mode, ...patch } : mode,
  );
  return { ...config, modes };
}

function blankRole(): GameRole {
  return { name: "", required: false };
}

function blankRank(): GameRank {
  return { name: "", value: 0 };
}

export function GameConfigEditor({
  config,
  onChange,
  validationError,
}: GameConfigEditorProps) {
  function addMode() {
    onChange({
      ...config,
      modes: [
        ...config.modes,
        {
          name: "",
          slots: 1,
          party_size_min: 1,
          party_size_max: 1,
          roles_required: false,
          rank_required: false,
          roles: [],
          ranks: [],
        },
      ],
    });
  }

  function removeMode(index: number) {
    onChange({
      ...config,
      modes: config.modes.filter((_, i) => i !== index),
    });
  }

  return (
    <fieldset className="game-config-editor">
      <legend>Matchmaking configuration</legend>
      <label>
        Genre (optional)
        <input
          value={config.genre ?? ""}
          onChange={(event) =>
            onChange({ ...config, genre: event.target.value })
          }
          autoComplete="off"
        />
      </label>
      <label>
        Platforms (comma-separated)
        <input
          value={formatListInput(config.platforms)}
          onChange={(event) =>
            onChange({
              ...config,
              platforms: parseListInput(event.target.value),
            })
          }
          placeholder="pc, console"
          autoComplete="off"
        />
      </label>
      <label>
        Regions
        <input
          value={formatListInput(config.regions)}
          onChange={(event) =>
            onChange({
              ...config,
              regions: parseListInput(event.target.value),
            })
          }
          required
          autoComplete="off"
        />
      </label>

      {config.modes.map((mode, index) => (
        <article key={`mode-${index}`} className="game-mode-card">
          <header className="game-mode-card__header">
            <h3>Mode {index + 1}</h3>
            {config.modes.length > 1 ? (
              <button type="button" onClick={() => removeMode(index)}>
                Remove mode
              </button>
            ) : null}
          </header>
          <label>
            Mode name
            <input
              value={mode.name}
              onChange={(event) =>
                onChange(
                  updateMode(config, index, { name: event.target.value }),
                )
              }
              required
              autoComplete="off"
            />
          </label>
          <div className="game-mode-grid">
            <label>
              Slots
              <input
                type="number"
                min={1}
                value={mode.slots}
                onChange={(event) =>
                  onChange(
                    updateMode(config, index, {
                      slots: Number(event.target.value) || 0,
                    }),
                  )
                }
              />
            </label>
            <label>
              Party min
              <input
                type="number"
                min={1}
                value={mode.party_size_min}
                onChange={(event) =>
                  onChange(
                    updateMode(config, index, {
                      party_size_min: Number(event.target.value) || 0,
                    }),
                  )
                }
              />
            </label>
            <label>
              Party max
              <input
                type="number"
                min={1}
                value={mode.party_size_max}
                onChange={(event) =>
                  onChange(
                    updateMode(config, index, {
                      party_size_max: Number(event.target.value) || 0,
                    }),
                  )
                }
              />
            </label>
          </div>
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={mode.roles_required}
              onChange={(event) =>
                onChange(
                  updateMode(config, index, {
                    roles_required: event.target.checked,
                  }),
                )
              }
            />
            Roles required
          </label>
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={mode.rank_required}
              onChange={(event) =>
                onChange(
                  updateMode(config, index, {
                    rank_required: event.target.checked,
                  }),
                )
              }
            />
            Rank required
          </label>

          <section className="game-mode-subsection">
            <h4>Roles</h4>
            {mode.roles.map((role, roleIndex) => (
              <div key={`role-${index}-${roleIndex}`} className="inline-row">
                <input
                  value={role.name}
                  placeholder="Role name"
                  onChange={(event) => {
                    const roles = mode.roles.map((item, i) =>
                      i === roleIndex
                        ? { ...item, name: event.target.value }
                        : item,
                    );
                    onChange(updateMode(config, index, { roles }));
                  }}
                  autoComplete="off"
                />
                <label className="checkbox-row">
                  <input
                    type="checkbox"
                    checked={role.required}
                    onChange={(event) => {
                      const roles = mode.roles.map((item, i) =>
                        i === roleIndex
                          ? { ...item, required: event.target.checked }
                          : item,
                      );
                      onChange(updateMode(config, index, { roles }));
                    }}
                  />
                  Required
                </label>
                <button
                  type="button"
                  onClick={() => {
                    const roles = mode.roles.filter((_, i) => i !== roleIndex);
                    onChange(updateMode(config, index, { roles }));
                  }}
                >
                  Remove
                </button>
              </div>
            ))}
            <button
              type="button"
              onClick={() =>
                onChange(
                  updateMode(config, index, {
                    roles: [...mode.roles, blankRole()],
                  }),
                )
              }
            >
              Add role
            </button>
          </section>

          <section className="game-mode-subsection">
            <h4>Ranks</h4>
            {mode.ranks.map((rank, rankIndex) => (
              <div key={`rank-${index}-${rankIndex}`} className="inline-row">
                <input
                  value={rank.name}
                  placeholder="Rank name"
                  onChange={(event) => {
                    const ranks = mode.ranks.map((item, i) =>
                      i === rankIndex
                        ? { ...item, name: event.target.value }
                        : item,
                    );
                    onChange(updateMode(config, index, { ranks }));
                  }}
                  autoComplete="off"
                />
                <input
                  type="number"
                  value={rank.value}
                  aria-label="Rank value"
                  onChange={(event) => {
                    const ranks = mode.ranks.map((item, i) =>
                      i === rankIndex
                        ? { ...item, value: Number(event.target.value) || 0 }
                        : item,
                    );
                    onChange(updateMode(config, index, { ranks }));
                  }}
                />
                <button
                  type="button"
                  onClick={() => {
                    const ranks = mode.ranks.filter((_, i) => i !== rankIndex);
                    onChange(updateMode(config, index, { ranks }));
                  }}
                >
                  Remove
                </button>
              </div>
            ))}
            <button
              type="button"
              onClick={() =>
                onChange(
                  updateMode(config, index, {
                    ranks: [...mode.ranks, blankRank()],
                  }),
                )
              }
            >
              Add rank
            </button>
          </section>
        </article>
      ))}

      <button type="button" onClick={addMode}>
        Add mode
      </button>
      {validationError ? (
        <p role="alert" className="error">
          {validationError}
        </p>
      ) : null}
    </fieldset>
  );
}
