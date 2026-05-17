# Fightris

Head-to-head terminal Tetris with a powerup economy.

A by-memory recreation of **Battletris**, a GUI game written for Solaris in the
Brown University CS department in the 1990s.

## IMPORTANT GLOBAL CONTEXT

This is a *personal learning project*.  The whole point is to get
better at golang and programming in general, so always explain your
rationale behind any design decisions before implementing in code.

## Usage

```
fightris -1p     # single player, arrow keys + space
fightris -2p     # two players, one terminal (WASD+E vs arrows+space)
```

## Current State

Working two-player local versus mode:

- All 7 tetrominoes with correct SRS rotation tables and wall kicks
- Standard Guideline scoring (100/300/500/800 × level)
- Level progression every 10 lines
- Ghost piece rendering (gray, distinct from colored active piece)
- 7-bag randomizer (`game/bag.go`) — guarantees all 7 types before any repeats
- Piece colors — standard Guideline palette via `pieceColors` table in renderer
- Next-piece preview panel to the right of each board
- Data-driven keybind system: two `Keymap` structs (key map + rune map per player),
  case-insensitive rune lookup; adding a binding is one line
- `-1p` / `-2p` mode flags; `run1P` / `run2P` are separate loop functions
- Origin-parameterized renderer — two boards side by side is
  `Draw(s, p1, 0, 0)` + `Draw(s, p2, TotalWidth+gap, 0)`
- Single lock+spawn path so lock-event hooks fire exactly once per piece
- Lock delay — 500ms grace window after landing; any move/rotate resets the timer
  (no 15-reset Guideline cap yet); hard drop still locks instantly
- **Garbage lines**: clearing 2/3/4 lines sends 1/2/4 rows of junk to the opponent;
  each garbage row is fully filled except one random escape hole; rendered gray
- Winner overlay + keypress-to-exit at game end
- CW rotation only (no CCW)

## Immediate TODOs

1. QOL: keybinding config file

## Milestone 2: Two-player LAN deathmatch

- Two `game.State` objects, one local one remote
- Serialize/deserialize `State` deltas (lines cleared → garbage rows sent to opponent)
- Deathmatch: last player alive wins the match

## Side Quest - Serverless matchmaker package

- NAT traversal for P2P sessions
- off the shelf?
- geek-only one-shot delete-after-use AWS deploy package ("just provide your IPs")

## Milestone 3: Rounds mode

- Match is N rounds; round ends when one player tops out
- Round winner earns round points; match winner is highest cumulative round points
- Between rounds: brief results screen, then store

## Milestone 4: Store and powerup system

Powerups are purchased between rounds with in-round currency (lines cleared = coins).
Activated via a dedicated key during gameplay; effect applied to self or opponent.
**The store has a countdown timer** — decision must be made under pressure; no pausing
the match to deliberate.

**Architecture hook:** `InputHandler` already has a comment marking where to add a
`[]Filter` pipeline (`Filter func(Action) Action`). Powerups that affect controls
(binding reversal, drop effects) go there. Board-affecting powerups (extra garbage rows,
display flip) get callbacks on `State`.

**Powerup ideas:**

- Unlock CCW rotation for yourself (permanently for the match, or N pieces)
- Swap your next piece queue with opponent's
- Reverse/rotate opponent's input bindings for N pieces
- Flip or rotate opponent's display
- Opponent's active piece auto-rotates one step per gravity tick
- Force yourself an I piece next
- User-defined plugins (mod API TBD)
- Standard Tetris Hold implementation

## Architecture Notes

- `game/state.go` — all game logic; no I/O; `AddGarbage(n int) bool` sends garbage rows
- `game/piece/` — piece geometry and SRS kick tables
- `game/board/` — grid, collision, row clearing; `board.Garbage = 8` for garbage cells
- `game/input.go` — `Action` enum + `InputHandler`; future powerup filters slot in here
- `render/render.go` — tcell renderer; `Draw(s, st, originX, originY int)`;
  `TotalWidth` exported so callers know where to place a second board
- `main.go` — `Keymap` struct + per-player keymaps; `run1P` / `run2P` game loops;
  per-player lock timer arrays; garbage routed through `lockAndSpawn`
