# Fightris

Head-to-head networked Tetris with a powerup economy.

## IMPORTANT GLOBAL CONTEXT

This is a *personal learning project*.  The whole point is to get
better at golang and programming in general, so always explain your
rationale behind any design decisions before implementing in code.

## Current State

Working single-player Tetris:

- All 7 tetrominoes with correct SRS rotation tables and wall kicks
- Standard Guideline scoring (100/300/500/800 × level)
- Level progression every 10 lines
- Ghost piece rendering (gray, distinct from colored active piece)
- 7-bag randomizer (`game/bag.go`) — guarantees all 7 types before any repeats
- Piece colors — standard Guideline palette via `pieceColors` table in renderer
- Next-piece preview panel to the right of the board
- Action-based input pipeline decoupled from tcell (ready for powerup filters)
- Origin-parameterized renderer — two boards side by side is
  `Draw(s, p1, 0, 0)` + `Draw(s, p2, PanelWidth+gap, 0)`
- Single lock+spawn path so lock-event hooks fire exactly once per piece
- Lock delay — 500ms grace window after landing; any move/rotate resets the timer
  (no 15-reset Guideline cap yet); hard drop still locks instantly

## Immediate TODOs (single-player complete)

1. QOL: keybinding configfile

## Side Quest - Two Players, One Terminal

- 😒
- One player has wasd, the other has arrows.

## Milestone 2: Two-player LAN deathmatch

- Two `game.State` objects, one local one remote
- Serialize/deserialize `State` deltas (lines cleared → garbage rows sent to opponent)
- Garbage rows: fill a row with a fixed cell color and one random gap
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
- Standard Tetris Hold implementation

## Architecture Notes

- `game/state.go` — all game logic; no I/O
- `game/piece/` — piece geometry and SRS kick tables
- `game/board/` — grid, collision, row clearing
- `game/input.go` — `Action` enum + `InputHandler`; future powerup filters slot in here
- `render/render.go` — tcell renderer; `Draw(s, st, originX, originY int)`
- `main.go` — event loop, gravity ticker, single `lockAndSpawn` path
