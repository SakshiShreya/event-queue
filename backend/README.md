# Backend — Virtual Waiting Room API

A virtual waiting room (Queue-it / Cloudflare Waiting Room style). Visitors enter, get a signed token with a sequence number, poll `status`, and are admitted at a controlled rate. There is no per-visitor database row: a visitor's position is just `number - admitted_up_to`.

**Stack:** Go · [chi](https://github.com/go-chi/chi) · Turso (cloud SQLite, via `libsql-client-go`) · `rs/cors` · polling (no WebSockets). Redis is planned for the hot counters (see Roadmap).

## Status

| Area                                                          | State                                 |
| ------------------------------------------------------------- | ------------------------------------- |
| `POST /rooms/{id}/enter`                                      | Done                                  |
| `GET /rooms/{id}/status` + local cache                        | Done                                  |
| Admitter goroutine                                            | Done (hard-coded to room `demo`)      |
| HMAC queue tokens                                             | Done, with tests                      |
| Admin API (snapshot, rate, pause, open, close, admit)         | Done, protected by `X-Admin-Key`      |
| Redis `CounterStore`, leader lock                             | Next                                  |
| Pass tokens, `/verify`, React page                            | Planned                               |
| `LoadRoom`, `SaveRoom`, `SaveCounters`, `AcquireAdmitterLock` | Stubs until the Redis work            |
| Turso store tests                                             | Skipped by choice (optional, Phase H) |

Room reset was dropped on purpose: for a new event, create a new room.

## Setup

1. Create a Turso database and get its URL and auth token ([Turso docs](https://docs.turso.tech)).
2. Create `backend/.env` (gitignored, never commit it):

```
TURSO_DATABASE_URL=libsql://<your-db>.turso.io
TURSO_AUTH_TOKEN=<your-token>
QUEUE_SECRET=<random string, 32+ characters>
ADMIN_KEY=<random string, 32+ characters>
```

Generate secrets with `openssl rand -hex 32`. The server exits at startup if `QUEUE_SECRET` or `ADMIN_KEY` is missing or too short.

3. Run from the `backend` directory:

```bash
go run ./cmd/server
```

The server listens on `http://localhost:8080`. On startup it connects to Turso, runs the idempotent migration (which seeds a `demo` room) and starts the cache refresher and the admitter. CORS allows `http://localhost:5173` (the Vite dev server). `Ctrl-C` shuts down gracefully.

```bash
curl localhost:8080/health
go test ./...
```

## Try it

```bash
# join the room; note the token and number
curl -s -X POST localhost:8080/rooms/demo/enter

# poll status with your token
curl -s -H "X-Queue-Token: <token>" localhost:8080/rooms/demo/status

# admin: look at the room, speed it up, let 10 people in now
curl -s -H "X-Admin-Key: $ADMIN_KEY" localhost:8080/admin/rooms/demo
curl -s -X POST -H "X-Admin-Key: $ADMIN_KEY" -d '{"perMin":120}' localhost:8080/admin/rooms/demo/rate
curl -s -X POST -H "X-Admin-Key: $ADMIN_KEY" -d '{"n":10}' localhost:8080/admin/rooms/demo/admit
```

`status` reads from a cache that refreshes every second, so allow about a second for admin changes to show up there.

## Project layout

```
backend/
├── cmd/server/
│   ├── main.go            # wiring, router, middleware, graceful shutdown
│   ├── handlers.go        # public handlers: enter, status
│   └── admin_handlers.go  # admin handlers
└── internal/
    ├── admitter/          # ticker loop that advances admitted_up_to by rate
    ├── roomcache/         # per-room RoomState cache, refreshed every second
    ├── db/                # opens Turso, runs migrations (schema.sql embedded)
    ├── store/             # CounterStore / RoomRepo interfaces, types, sentinel errors
    │   └── turso/         # Turso implementation
    └── token/             # HMAC-SHA256 sign/verify (+ tests)
```

Handlers depend on the `store.CounterStore` interface, so the Turso store can be swapped for a Redis store without touching them.

## Endpoints

### Public

| Method | Path                 | Description                                                                      |
| ------ | -------------------- | -------------------------------------------------------------------------------- |
| GET    | `/health`            | Health check                                                                     |
| POST   | `/rooms/{id}/enter`  | Join the room. Returns `{token, number, retryAfter}`                             |
| GET    | `/rooms/{id}/status` | Requires `X-Queue-Token`. Returns `{state, ahead, retryAfter, estimatedWaitSec}` |

**enter:** if the request carries a valid token for this room, the same token and number come back (no double-join). A missing, invalid or other-room token is treated as a new visitor. Closed rooms return 409 and unknown rooms 404.

**status:** `state` is `waiting`, `admitted`, `paused` or `closed`. `ahead = max(0, number - admitted_up_to)`. `retryAfter` is 5–7 seconds (base plus jitter). `estimatedWaitSec` is `ahead * 60 / rate`, or `-1` when the rate is 0. A missing or invalid token, or one for another room, returns 401.

### Admin

All admin routes require the header `X-Admin-Key: <ADMIN_KEY>` and return 401 without it. Every route (except where noted) returns the room snapshot:

```json
{
  "status": "open",
  "admitRatePerMin": 120,
  "nextNumber": 57,
  "admittedUpTo": 40
}
```

| Method | Path                      | Body              | Description                                                                                                             |
| ------ | ------------------------- | ----------------- | ----------------------------------------------------------------------------------------------------------------------- |
| GET    | `/admin/rooms/{id}`       | none              | Current room state, read straight from the store (not the cache)                                                        |
| POST   | `/admin/rooms/{id}/rate`  | `{"perMin": 120}` | Set the admit rate. Must be greater than 0                                                                              |
| POST   | `/admin/rooms/{id}/pause` | none              | Pause admissions. 409 if the room is closed                                                                             |
| POST   | `/admin/rooms/{id}/open`  | none              | Set the room to open. Also reopens a closed room                                                                        |
| POST   | `/admin/rooms/{id}/close` | none              | Close the room                                                                                                          |
| POST   | `/admin/rooms/{id}/admit` | `{"n": 10}`       | Admit up to n people now, capped at the number waiting. `n` must be greater than 0. 409 if the room is paused or closed |

### Errors

Every error is JSON: `{"error": "message"}`.

| Status | Meaning                                                                                   |
| ------ | ----------------------------------------------------------------------------------------- |
| 400    | Validation or bad request (bad JSON, `n` or `perMin` not greater than 0)                  |
| 401    | Missing or invalid token or admin key                                                     |
| 404    | Room not found                                                                            |
| 409    | Room closed, or not open (for `admit`), or `pause` on a closed room                       |
| 413    | Request body over 4 KB                                                                    |
| 500    | Unexpected error. The real error is logged and the client gets a generic `internal error` |

## How it works

**Tokens.** `enter` increments `next_number` atomically and returns an HMAC-SHA256 signed token `{room_id, number, issued_at}`. `status` verifies it locally with no database call. All instances must share `QUEUE_SECRET`.

**Atomic counters.** `NextNumber` and `Admit` are each a single `UPDATE ... RETURNING` statement. `Admit` is capped with `MIN(admitted_up_to + n, next_number)` and only applies to open rooms. `SetRate` and `SetStatus` are single `UPDATE`s that return `ErrNotFound` when no row matches.

**Status never queries the DB per poll.** `roomcache` holds the latest `RoomState` per room behind a `sync.RWMutex`. It fills on first request and a ticker goroutine refreshes it every second. Known gap: an unknown room id hits Turso on every request (add a short negative cache later).

**Admitter.** Every second it reads a snapshot, skips paused or closed rooms and a rate of 0, accumulates fractional "people credit" from the elapsed time, and admits `floor(credit)` capped at the number waiting. If `Admit` fails, the credit is kept and retried next tick. It stops on context cancellation.

**Admin auth.** `authMiddleware` compares `X-Admin-Key` with the configured key using a constant-time comparison. The key is read once at startup.

**Errors map to status codes in one place** (`writeError`), via sentinel errors and `errors.Is`.

**Known limitation.** `pause` and `admit` read the room status and then act, so a status change in between could slip through. Fine for now.

## Roadmap

1. Redis `CounterStore` (`INCR`, atomic capped `Admit` via Lua), hydrate from Turso on boot, periodic flush to Turso, `COUNTER_STORE=turso|redis` flag.
2. Admitter leader lock (`SET NX PX` with renewal) and a two-instance drill.
3. Pass tokens, `POST /verify`, then the React waiting page in `frontend/`.
4. Optional: Turso store tests (50-goroutine `NextNumber` test), load test comparing Turso vs. Redis.
   The full plan lives in the project doc `queue-app-plan.md`.
