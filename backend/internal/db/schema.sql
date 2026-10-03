CREATE TABLE IF NOT EXISTS Queue (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS Ticket (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    queue_id TEXT NOT NULL REFERENCES Queue(id),
    display_name TEXT NOT NULL,
    party_size INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'called', 'serving', 'done', 'skipped')),
    joined_at INTEGER NOT NULL,
    called_at INTEGER,
    done_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_ticket_queueid_status_id ON Ticket (queue_id, status, id);
