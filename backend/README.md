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

The server listens on `http://localhost:8080`. On startup it connects to Turso, creates any missing tables, and ensures a `default` queue exists. CORS allows requests from `http://localhost:5173` (the Vite dev server).

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
        ├── ticket.go         # Ticket type, status constants, sentinel errors
        ├── store.go          # Store interface the handlers depend on
        └── dbstore.go        # Turso-backed implementation of Store
```

Handlers only talk to the `Store` interface, so the storage layer can change without touching `main.go`.

## Endpoints

| Method | Path            | Body                                | Description                                          |
| ------ | --------------- | ----------------------------------- | ---------------------------------------------------- |
| GET    | `/health`       | none                                | Health check                                         |
| POST   | `/join`         | `{"name": "...", "party_size": 1}`  | Join the queue; returns the new ticket               |
| GET    | `/queue`        | none                                | All tickets plus a count                             |
| GET    | `/tickets/{id}` | none                                | One ticket, its live position, and the waiting count |
| POST   | `/call`         | none                                | Calls the next waiting ticket (earliest to join)     |
| POST   | `/skip`         | `{"ticket_id": "..."}`              | Skips a waiting or called ticket                     |
| POST   | `/serve`        | `{"ticket_id": "..."}`              | Marks a called ticket as done                        |

### Ticket

```json
{
  "id": "12",
  "name": "Sam",
  "position": 3,
  "status": "waiting",
  "joined_at": 1790000000,
  "party_size": 2
}
```

`joined_at` is a Unix timestamp in seconds. `position` is 1-based for waiting tickets and `0` for every other status.

### Validation (`/join`)

- `name` is trimmed of surrounding whitespace, must not be empty, and is limited to 50 characters.
- `party_size` must be between 1 and 20.

### Errors

Every error is returned as JSON: `{"error": "message"}`.

| Status | Meaning                                                             |
| ------ | ------------------------------------------------------------------- |
| 400    | Invalid JSON, missing `ticket_id`, or failed validation             |
| 404    | Ticket doesn't exist (including ids that aren't numbers)            |
| 409    | Action not allowed in the ticket's current state, or nobody to call |
| 500    | Unexpected server or database error                                 |

## Ticket lifecycle

```
waiting ──call──▶ called ──serve──▶ done
   │                 │
   └──────skip───────┴──▶ skipped
```

Only these transitions are allowed. Anything else returns `409`.

## How it works

**Position is never stored.** A waiting ticket's position is the number of tickets in the same queue with status `waiting` and an id at or below its own, counted on every read. Because ids only increase, id order is join order.

**Transitions are atomic.** Each state change is a single `UPDATE` guarded by the required current status (for example, `serve` only matches rows where `status = 'called'`). If no row matches, the code looks the ticket up to decide between `404` and `409`. That makes concurrent requests safe without any locking in Go.

**Errors map to status codes in one place.** The store returns sentinel errors (`ErrValidation`, `ErrNotFound`, `ErrConflict`), wrapped with context, and `writeError` in `main.go` translates them with `errors.Is`.

## History

The first version kept the queue in memory using a Fenwick tree for position tracking. It was removed once Turso persistence was in place. It still exists under the git tag `week7-inmemory`.

## Planned

- A `serving` status between `called` and `done`, with a separate endpoint to finish
- Admin endpoints: reorder, reset, stats
- Timestamps for `called_at` and `done_at` exposed in the API