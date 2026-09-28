CREATE TABLE Queue (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE Ticket (
    id TEXT PRIMARY KEY,
    queue_id TEXT NOT NULL REFERENCES Queue(id),
    display_name TEXT NOT NULL,
    party_size INTEGER,
    status TEXT NOT NULL,
    position INTEGER,
    joined_at TEXT NOT NULL,
    called_at TEXT,
    done_at TEXT
);
