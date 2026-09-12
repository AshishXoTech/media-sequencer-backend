-- Schema for the Multi-Window Media Sequencer.
--
-- media_items is intentionally NOT scoped to a window (no window_id
-- column) — see the comment on models.MediaItem for why: sync must
-- be able to show any item in any window, including ones that don't
-- normally play it.

CREATE TABLE IF NOT EXISTS windows (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS media_items (
    id               TEXT PRIMARY KEY,
    label            TEXT NOT NULL,
    type             TEXT NOT NULL CHECK (type IN ('image', 'video', 'blank')),
    url              TEXT NOT NULL DEFAULT '',
    duration_seconds INTEGER NOT NULL CHECK (duration_seconds > 0),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS playlist_entries (
    id            TEXT PRIMARY KEY,
    window_id     TEXT NOT NULL REFERENCES windows(id) ON DELETE CASCADE,
    media_item_id TEXT NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
    position      INTEGER NOT NULL,
    UNIQUE (window_id, position)
);

-- Single-row table holding the one currently-active sync override,
-- if any. A real multi-tenant system might key this by "session" or
-- "screen group," but the brief describes one global sync action
-- affecting every window, so one row is the correct, non-over-
-- engineered model for this spec.
CREATE TABLE IF NOT EXISTS sync_state (
    id               INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    media_item_id    TEXT REFERENCES media_items(id) ON DELETE SET NULL,
    started_at       TIMESTAMPTZ,
    duration_seconds INTEGER
);