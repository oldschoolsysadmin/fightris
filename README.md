# Fightris

Head-to-head terminal Tetris with a powerup economy.

A by-memory recreation of **Battletris**, a GUI game written for Solaris at the
Brown University Computer Science department during the 1990s.

## IMPORTANT GLOBAL CONTEXT

This is a *personal learning project*.  The whole point is to get
better at golang and programming in general, so always explain your
rationale behind any design decisions before implementing in code.

## Usage

```
fightris -1p                    # single player, arrow keys + space  # TODO: make this the default
fightris -2p                    # two players, one terminal (WASD+E vs arrows+space)
fightris -host [-port N]        # host a LAN match, wait for a joiner
fightris -join <addr> [-spam N] # join a LAN match (addr like 192.168.1.5:4000; :4000 assumed if no port)
```

All runs write a verbose event log to `fightris.log` in the working directory
(timer ticks, locks, garbage, network packets, panics with stack traces) — tcell
owns the terminal, so logs can't go to stdout.

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
- Single lock+spawn path (`engine.lockAndSpawn`) so lock-event hooks fire exactly once per piece
- Lock delay — 500ms grace window after landing; any move/rotate resets the timer
  (no 15-reset Guideline cap yet); hard drop still locks instantly
- **Garbage lines**: clearing 2/3/4 lines sends 1/2/4 rows of junk to the opponent;
  each garbage row is fully filled except one random escape hole; rendered gray
- Winner overlay + keypress-to-exit at game end
- CW rotation only (no CCW)

## Milestone 2: Two-player LAN deathmatch

**Status: first playable cut landed (`netplay/` package + `runLAN`).** Verified by
unit tests (snapshot/effect round-trip, localhost handshake); full match needs two
real terminals to exercise the loop.

Design — **authoritative-local + snapshots** (not lockstep):

- Each machine simulates ONLY its own board, authoritatively. Two `game.State`
  objects per process: one local (simulated) and one remote (a render-only shadow
  overwritten by incoming snapshots, never simulated). Consequence: **no seeded bag
  needed** — that's a lockstep-only requirement.
- Protocol: UDP from day one — LAN is direct, internet (Side Quest) just adds a
  connection-establishment step on top of the same handshake.
- Two message types, two delivery semantics:
  - **Snapshot** — sender's board for the peer to render. Latest-wins (drop `seq` ≤
    last seen); idempotent, so a lost snapshot is simply superseded.
  - **Effect** — an apply-once command the receiver runs against its OWN board,
    deduped by payload ID. Garbage is the first effect; **only the row count crosses
    the wire** (the receiver picks the hole locally). Effects are the powerup
    extension seam (see M4) — new opponent-affecting powerups add an Effect *kind*,
    not a new packet type.
- Reliability: send N redundant copies of each packet; receiver dedups (snapshots by
  seq, effects by id). No ACK/retransmit. `-spam N` flag (default 1, raise on lossy
  links) controls redundancy.
- Deathmatch: `GameOver` rides in the snapshot; the dier keeps broadcasting it, the
  survivor wins.

## Side Quest - Serverless matchmaker package

Geek-only, one-shot AWS deploy: spin up, play, auto-teardown. Two options:

**Option A: Lambda signaling + UDP hole punching**
- API Gateway + Lambda acts as a one-time rendezvous: each player posts their public
  IP/port (Lambda can read it from the request), fetches the peer's, then both attempt
  UDP hole punching simultaneously
- Cost: $0 (free tier); deploys with `cdk deploy`, cleans up via TTL
- Limitation: fails on symmetric NATs (CGNAT, some corporate routers) — fine for
  "two geeks who know their setup", not universally reliable

**Option B: EC2 t4g.nano TURN relay**
- One-command CloudFormation stack spins up a tiny instance running `coturn`
- All game traffic relays through it — works against any NAT type
- Cost: ~$0.004/hr; terminate the stack when the session ends
- More infrastructure, but bulletproof

**UX sketch (both options):**
- `fightris -host` — deploys the stack, prints a short join code for the other player
- `fightris -join <code>` — fetches peer address, connects
- Stack tears itself down on game end (or timeout)

## Milestone 3: Rounds mode

- Match is N rounds; round ends at gravity increase timer
- Match ends when one player tops out.
- Between rounds: brief results screen, then store

## Milestone 4: Store and powerup system

Powerups are purchased between rounds with in-round currency (lines cleared = coins).
Assigned to a toolbar, actived with tool key (ie. 1-5); effect applied to self or opponent.
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
- `engine/engine.go` — wraps `game.State` with gravity + lock-delay timers; exposes
  `HandleAction` / `HandleGravity` / `HandleLock`, each returning `(cleared int, alive bool)`;
  garbage routing is intentionally left to the caller so LAN can send a message instead
  of calling `AddGarbage` directly
- `render/render.go` — tcell renderer; `Draw(s, st, originX, originY int)`;
  `TotalWidth` exported so callers know where to place a second board
- `netplay/` — UDP LAN transport. `Packet` envelope (hello/snapshot/effect),
  JSON-encoded; `Listen`/`Dial` do the handshake; `send` writes `-spam` redundant
  copies; `Run` reads in a goroutine, dedups (snapshot latest-wins, effect
  apply-once), and posts decoded packets into the tcell event queue as
  `netplay.Incoming` — so all `game.State` mutation stays on the single loop
  goroutine, no locks. `message.go` holds the wire types + `Snapshot.ApplyTo` (rebuild
  a shadow `State` for rendering) and the `Effect`/`GarbageEffect` powerup seam.
- `main.go` — `flag`-based mode select (`-1p`/`-2p`/`-host`/`-join`/`-port`/`-spam`);
  `Keymap` struct + per-player keymaps; `run1P` / `run2P` / `runLAN` game loops; log
  redirected to `fightris.log` and `recover()` in each loop dumps stack traces there.
  In `run2P` the `routeGarbage` closure is the local seam; `runLAN` is the network
  version (clears send a garbage Effect; the opponent board is a snapshot-fed shadow)
