-- Example seed data. The assignment brief asks for "seed data
-- matching the example windows and media lists" but the source
-- document contains no actual example table or diagram — this is
-- documented explicitly as an assumption in the README. The data
-- below is designed to demonstrate every required behavior:
--   - three windows, each with a different playlist
--   - image, video, AND an explicit blank item (Window 2)
--   - M2 deliberately reused in both Window 1 and Window 3, so that
--     syncing to M2 is visibly meaningful for two windows that
--     already play it and Window 2, which normally never does.

INSERT INTO media_items (id, label, type, url, duration_seconds) VALUES
    ('m1', 'M1', 'image', 'https://picsum.photos/seed/m1/800/450', 10),
    ('m2', 'M2', 'video', 'https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4', 15),
    ('m3', 'M3', 'image', 'https://picsum.photos/seed/m3/800/450', 10),
    ('m4', 'M4', 'video', 'https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.webm', 20),
    ('m5', 'M5', 'image', 'https://picsum.photos/seed/m5/800/450', 10),
    ('m6', 'M6', 'image', 'https://picsum.photos/seed/m6/800/450', 8),
    ('m7', 'M7', 'video', 'https://interactive-examples.mdn.mozilla.net/media/cc0-videos/flower.mp4', 12),
    ('blank', 'Blank', 'blank', '', 5)
ON CONFLICT (id) DO NOTHING;

INSERT INTO windows (id, name) VALUES
    ('w1', 'Window 1'),
    ('w2', 'Window 2'),
    ('w3', 'Window 3')
ON CONFLICT (id) DO NOTHING;

-- Window 1: M1 -> M2 -> M3
INSERT INTO playlist_entries (id, window_id, media_item_id, position) VALUES
    ('pe1', 'w1', 'm1', 0),
    ('pe2', 'w1', 'm2', 1),
    ('pe3', 'w1', 'm3', 2)
ON CONFLICT (id) DO NOTHING;

-- Window 2: M4 -> Blank -> M5
INSERT INTO playlist_entries (id, window_id, media_item_id, position) VALUES
    ('pe4', 'w2', 'm4', 0),
    ('pe5', 'w2', 'blank', 1),
    ('pe6', 'w2', 'm5', 2)
ON CONFLICT (id) DO NOTHING;

-- Window 3: M2 -> M6 -> M7
INSERT INTO playlist_entries (id, window_id, media_item_id, position) VALUES
    ('pe7', 'w3', 'm2', 0),
    ('pe8', 'w3', 'm6', 1),
    ('pe9', 'w3', 'm7', 2)
ON CONFLICT (id) DO NOTHING;