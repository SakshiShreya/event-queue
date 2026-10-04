# Backend — Go HTTP API

Waitlist queue API. People join a line, an admin calls them in order, and every ticket's position is computed live from the database.

**Stack:** Go · [chi](https://github.com/go-chi/chi) router · Turso (cloud SQLite) via `libsql-client-go` · CORS via `rs/cors`

## Setup

1. Create a Turso database and grab its URL and an auth token (see [Turso docs](https://docs.turso.tech)).
2. Create `backend/.env` (it is gitignored, never commit it):

```
   TURSO_DATABASE_URL=libsql://<your-db>.turso.io
   TURSO_AUTH_TOKEN=<your-token>
```

3. Run the server from the `backend` directory:

```bash
   go run ./cmd/server
```

The server listens on `http://localhost:8080`. On startup it connects to Turso and creates any missing tables. CORS allows requests from `http://localhost:5173` (the Vite dev server).

Quick check:

```bash
curl http://localhost:8080/health
```

## Project layout

```
backend/
├── cmd/server/main.go        # router, middleware, HTTP handlers
└── internal/
    ├── db/
    │   ├── db.go             # opens the connection, runs migrations
    │   └── schema.sql        # table definitions (embedded into the binary)
    └── queue/
        ├── ticket.go         # Ticket type, status constants, limits, sentinel errors
        ├── store.go          # Store interface the handlers depend on
        ├── dbstore.go        # Turso-backed implementation of Store
        └── fenwick.go        # Fenwick tree (learning exercise, not used by the server)
```

Handlers only talk to the `Store` interface, so the storage layer can change without touching `main.go`.

## Endpoints

| Method | Path            | Body                               | Description                                          |
| ------ | --------------- | ---------------------------------- | ---------------------------------------------------- |
| GET    | `/health`       | none                               | Health check                                         |
| POST   | `/join`         | `{"name": "...", "party_size": 1}` | Join the queue; returns the new ticket (`201`)       |
| GET    | `/queue`        | none                               | All tickets plus a count                             |
| GET    | `/tickets/{id}` | none                               | One ticket, its live position, and the waiting count |
| POST   | `/call`         | none                               | Calls the next waiting ticket (earliest to join)     |
| POST   | `/start`        | `{"ticket_id": "..."}`             | Called ticket starts being served                    |
| POST   | `/skip`         | `{"ticket_id": "..."}`             | Skips a waiting or called ticket                     |
| POST   | `/done`         | `{"ticket_id": "..."}`             | Finishes a ticket that is being served               |

Action endpoints (`/start`, `/skip`, `/done`) return `{"success": true}` on success.

### Ticket

```json
{
  "id": "12",
  "name": "Sam",
  "position": 0,
  "status": "done",
  "joined_at": 1790000000,
  "called_at": 1790000300,
  "done_at": 1790000900,
  "party_size": 2
}
```

- `joined_at`, `called_at` and `done_at` are Unix timestamps in seconds.
- `called_at` and `done_at` are left out of the response until they are set.
- `position` is 1-based for waiting tickets and `0` for every other status.

### Validation (`/join`)

- `name` is trimmed of surrounding whitespace, must not be empty, and is limited to 50 characters.
- `party_size` is optional. If it is missing or `0` it defaults to `1`. Otherwise it must be between 1 and 20.
- Request bodies are limited to 4 KB.

### Errors

Every error is returned as JSON: `{"error": "message"}`.

| Status | Meaning                                                             |
| ------ | ------------------------------------------------------------------- |
| 400    | Invalid JSON, missing `ticket_id`, or failed validation             |
| 404    | Ticket doesn't exist (including ids that aren't numbers)            |
| 409    | Action not allowed in the ticket's current state, or nobody to call |
| 413    | Request body is larger than 4 KB                                    |
| 500    | Unexpected server or database error                                 |

## Ticket lifecycle

```
waiting ──call──▶ called ──start──▶ serving ──done──▶ done
   │                 │
   └──────skip───────┴──▶ skipped
```

| Action | Allowed from      | Moves to  |
| ------ | ----------------- | --------- |
| call   | waiting           | called    |
| start  | called            | serving   |
| done   | serving           | done      |
| skip   | waiting, called   | skipped   |

Any other transition returns `409`. A ticket that is already being served can't be skipped; it can only be finished.

## How it works

**Position is never stored.** A waiting ticket's position is the number of tickets in the same queue with status `waiting` and an id at or below its own, counted on every read. Because ids only increase, id order is join order.

**Transitions are atomic.** `start`, `skip` and `done` all go through one helper (`transition` in `dbstore.go`). It runs a single `UPDATE` that only matches the ticket if its current status is one of the allowed starting statuses. If no row matches, it looks the ticket up to decide between `404` and `409`. That makes concurrent requests safe without any locking in Go. `call` works the same way, picking the earliest waiting ticket inside one statement.

**Errors map to status codes in one place.** The store returns sentinel errors (`ErrValidation`, `ErrNotFound`, `ErrConflict`), wrapped with context, and `writeError` in `main.go` translates them with `errors.Is`.

## History

The first version kept the queue in memory and used a Fenwick tree for position tracking. That in-memory queue was removed once Turso persistence was in place. The tree itself is kept in `internal/queue/fenwick.go` as a learning exercise, and the full in-memory version is available under the git tag `week7-inmemory`.

## Planned

- Admin endpoints: reorder, reset, stats
- Wait time estimate based on `joined_at` and `called_at`