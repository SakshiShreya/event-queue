CREATE TABLE IF NOT EXISTS Rooms (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open' CHECK (status in ('open', 'paused', 'closed')),
    admit_rate_per_min INTEGER NOT NULL DEFAULT 50,
    next_number INTEGER NOT NULL DEFAULT 0,
    admitted_up_to INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
