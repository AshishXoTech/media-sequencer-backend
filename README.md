# Multi-Window Media Sequencer — Backend

Golang REST API powering a multi-window media playback system with
dynamic playlists and cross-window sync.

**Live API:** https://media-sequencer-backend-f03h.onrender.com
**Health check:** https://media-sequencer-backend-f03h.onrender.com/health
**Frontend:** https://media-sequencer-frontend.vercel.app
**Frontend repo:** https://github.com/AshishXoTech/media-sequencer-frontend

## Tech stack

- **Go**, standard library `net/http` (Go 1.22+ method/wildcard routing)
- **PostgreSQL** via Neon (free tier)
- No ORM — plain SQL via `database/sql` + `lib/pq`

## How playback works (read this first)

There is **no stored "current position" or running timer anywhere in
the system.** What a window is showing right now is computed fresh,
on every request, as a pure function of wall-clock time:

1. Take `elapsed = now - fixed_epoch` (epoch is a hardcoded constant,
   shared across all windows, so every window's 5-hour cycle boundary
   lands at the same wall-clock moments).
2. `cyclePosition = elapsed mod 5 hours` — this is what makes the
   playlist snap back to item 0 exactly every 5 hours, rather than
   drifting.
3. `positionInPlaylist = cyclePosition mod totalPlaylistDuration` —
   this loops the list repeatedly inside the 5-hour cycle.
4. Walk the playlist's items by cumulative duration to find which
   item that position falls inside, and how far into that item.

Because this is a pure calculation, not stored state: a server
restart, a browser refresh, or two people asking at the same instant
all get consistent, correct answers with zero synchronization code.

## How sync works

Sync is a single database row (`sync_state`): which media item, when
it started, how long it lasts. `GET /windows/{id}/now-playing` checks
this row FIRST:

- If a sync is active (current time is within its window), every
  window's `now-playing` returns that forced item, regardless of the
  window's own playlist.
- If not, the deterministic cycle calculation above runs as normal.

When the sync duration elapses, there is no "resume" step — the next
poll simply finds the sync inactive and falls through to the normal
calculation, which has been running the whole time under the hood.
A window's own playlist is never modified or paused by a sync; it's
only shadowed for the sync's duration.

## Local run

```bash
go mod tidy
export DATABASE_URL="postgres://user:pass@host/db?sslmode=require"
go run ./cmd/server
curl http://localhost:8080/health
```

## Run with Docker

```bash
docker build -t media-sequencer-backend .
docker run -p 8080:8080 -e DATABASE_URL="postgres://..." media-sequencer-backend
curl http://localhost:8080/health
```

## Database setup

Schema and seed data are plain SQL files in `migrations/`. Run them
against your Postgres instance in order (Neon's SQL Editor, or any
`psql` client):

migrations/001_schema.sql
migrations/002_seed.sql


## API

| Method | Endpoint | Auth | Purpose |
|---|---|---|---|
| GET | `/health` | No | Health check |
| GET | `/windows` | No | List all windows |
| GET | `/windows/{id}/now-playing` | No | What this window should show right now |
| POST | `/windows/{id}/media` | No | Append an existing media item to a window's playlist |
| GET | `/media` | No | List all global media items |
| POST | `/media` | No | Create a new global media item |
| POST | `/sync` | No | Trigger sync — force one item into every window |
| GET | `/sync` | No | Current sync status |

### `GET /windows/{id}/now-playing` response
```json
{
  "media_item": {
    "id": "m2",
    "label": "M2",
    "type": "video",
    "url": "https://...",
    "duration_seconds": 15
  },
  "position_in_item_seconds": 7,
  "synced": false
}
```

### `POST /media` request
```json
{ "label": "M8", "type": "image", "url": "https://...", "duration_seconds": 12 }
```
`type` must be `image`, `video`, or `blank`. `url` is required unless `type` is `blank`.

### `POST /windows/{id}/media` request
```json
{ "media_item_id": "m8" }
```
Appends to the end of the window's playlist.

### `POST /sync` request
```json
{ "media_item_id": "m2", "duration_seconds": 15 }
```

## Assumptions & tradeoffs

- **No example windows/media list was provided in the assignment
  brief.** Seed data (3 windows, 8 media items including one explicit
  Blank) was designed to demonstrate every required behavior,
  particularly reusing M2 across two windows so sync into a *third*,
  non-M2 window is visibly meaningful.
- **In-memory storage was NOT used** — Postgres was chosen instead,
  since deployment happens on a host (Render) with no persistent
  disk on the free tier, so true persistence requires an external
  database regardless.
- **Media items are modeled as a single global catalog**, not scoped
  per window. This is a deliberate design choice: the sync feature
  requires that any item be displayable in any window, including
  windows that don't normally play it — scoping media to a window
  would make that impossible without duplicating records.
- **Free-tier cold starts.** Both Render (backend host) and Neon
  (database) suspend on inactivity. The first request after a period
  of idleness can transiently fail while the underlying compute
  wakes up, even though the very next request succeeds. The
  `now-playing` endpoint retries once internally to absorb the common
  case; a request that still fails after retrying is surfaced
  honestly as an error rather than masked. This is a known limitation
  of free-tier hosting, not a defect in the playback or sync logic,
  both of which are unit-tested independently of any network/hosting
  concerns.
- **A sync duration longer than the target item's own duration** is
  allowed; the frontend loops video playback to cover this case
  rather than the backend capping the duration artificially.
- **No authentication.** Not required by the brief; all endpoints are
  public.