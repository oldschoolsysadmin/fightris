# Fightris

Head-to-head networked Tetris with a powerup economy.

## Current State

Working single-player Tetris:
- All 7 tetrominoes with correct SRS rotation tables and wall kicks
- Standard Guideline scoring (100/300/500/800 × level)
- Level progression every 10 lines
- Ghost piece rendering
- Action-based input pipeline decoupled from tcell (ready for powerup filters)
- Origin-parameterized renderer — two boards side by side is `Draw(s, p1, 0, 0)` + `Draw(s, p2, PanelWidth+gap, 0)`
- Single lock+spawn path so lock-event hooks fire exactly once per piece

## Immediate TODOs (single-player complete)

1. **7-bag randomizer** — replace `piece.I` hardcoding with a shuffled bag; expose `NextPiece` on `State` for the preview panel
2. **Next-piece preview** — render the upcoming piece in the side panel
3. **Piece colors** — `Cell` already stores `PieceType` (1–7); wire up a color table in the renderer
4. **Hold piece** — `ActionHold`, one swap per piece, stored on `State`
5. **Lock delay** — ~500 ms grace window for slides/spins after landing before auto-lock

## Milestone 2: Two-player deathmatch over TCP

- Two `game.State` objects, one local one remote
- Serialize/deserialize `State` deltas (lines cleared → garbage rows sent to opponent)
- Garbage rows: fill a row with a fixed cell color and one random gap
- Deathmatch: last player alive wins the match

## Milestone 3: Rounds mode

- Match is N rounds; round ends when one player tops out
- Round winner earns round points; match winner is highest cumulative round points
- Between rounds: brief results screen, then store

## Milestone 4: Store and powerup system

Powerups are purchased between rounds with in-round currency (lines cleared = coins).
Activated via a dedicated key during gameplay; effect applied to self or opponent.

**Architecture hook:** `InputHandler` already has a comment marking where to add a
`[]Filter` pipeline (`Filter func(Action) Action`). Powerups that affect controls
(binding reversal, CCW-on-drop) go there. Board-affecting powerups (garbage rows,
display flip) get callbacks on `State`.

**Powerup ideas:**
- Swap your next piece queue with opponent's
- Reverse/rotate opponent's input bindings for N pieces
- Flip or rotate opponent's display
- Opponent's active piece auto-rotates one step per gravity tick
- Force yourself an I piece next
- User-defined plugins (mod API TBD)

## Architecture Notes

- `game/state.go` — all game logic; no I/O
- `game/piece/` — piece geometry and SRS kick tables
- `game/board/` — grid, collision, row clearing
- `game/input.go` — `Action` enum + `InputHandler`; future powerup filters slot in here
- `render/render.go` — tcell renderer; `Draw(s, st, originX, originY int)`
- `main.go` — event loop, gravity ticker, single `lockAndSpawn` path
