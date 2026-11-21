# Backend — Virtual Waiting Room API

A virtual waiting room (Queue-it / Cloudflare Waiting Room style). Visitors enter, get a signed token with a sequence number, poll `status`, and are admitted at a controlled rate. There is no per-visitor database row: a visitor's position is just `number - admitted_up_to`.

**Stack:** Go · [chi](https://github.com/go-chi/chi) · Turso (cloud SQLite, via `libsql-client-go`) · `rs/cors` · polling (no WebSockets). Redis is planned for the hot counters (see Roadmap).

## Status

| Area                                   | State                                                        |
| -------------------------------------- | ------------------------------------------------------------ |
| `POST /rooms/{id}/enter`               | Done                                                         |
| `GET /rooms/{id}/status` + local cache | Done                                                         |
| Admitter goroutine                     | Done (hard-coded to room `demo`)                             |
| HMAC queue tokens                      | Done, with tests                                             |
| `SetRate`, `SetStatus` (Turso store)   | Done                                                         |
| `Reset`, admin HTTP endpoints          | Next                                                         |
| Redis `CounterStore`, leader lock      | Planned                                                      |
| Pass tokens, `/verify`, React page     | Planned                                                      |
| `LoadRoom`, `SaveRoom`, `SaveCounters` | Stubs returning "not implemented" until the Redis flush work |
| Turso store tests                      | Skipped by choice (optional, Phase H)                        |

## Setup

1. Create a Turso database and get its URL and auth token ([Turso docs](https://docs.turso.tech)).
2. Create `backend/.env` (gitignored, never commit it):

```
TURSO_DATABASE_URL=libsql://<your-db>.turso.io
TURSO_AUTH_TOKEN=<your-token>
QUEUE_SECRET=<random string, 32+ characters>
```

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
```

Until the admin endpoints exist, change the rate or status in the Turso shell:

```sql
UPDATE Rooms SET admit_rate_per_min = 300, status = 'open' WHERE id = 'demo';
SELECT next_number, admitted_up_to FROM Rooms WHERE id = 'demo';
```

## Project layout

```
backend/
├── cmd/server/
│   ├── main.go            # wiring, router, middleware, graceful shutdown
│   └── handlers.go        # enter and status handlers
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

| Method | Path                 | Description                                                                      |
| ------ | -------------------- | -------------------------------------------------------------------------------- |
| GET    | `/health`            | Health check                                                                     |
| POST   | `/rooms/{id}/enter`  | Join the room. Returns `{token, number, retryAfter}`                             |
| GET    | `/rooms/{id}/status` | Requires `X-Queue-Token`. Returns `{state, ahead, retryAfter, estimatedWaitSec}` |

**enter:** if the request carries a valid token for this room, the same token and number come back (no double-join). A missing, invalid or other-room token is treated as a new visitor. Closed rooms return 409 and unknown rooms 404.

**status:** `state` is `waiting`, `admitted`, `paused` or `closed`. `ahead = max(0, number - admitted_up_to)`. `retryAfter` is 5–7 seconds (base plus jitter). `estimatedWaitSec` is `ahead * 60 / rate`, or `-1` when the rate is 0. A missing or invalid token, or one for another room, returns 401.

### Errors

Every error is JSON: `{"error": "message"}`.

| Status | Meaning                                                                                   |
| ------ | ----------------------------------------------------------------------------------------- |
| 400    | Validation or bad request                                                                 |
| 401    | Missing, invalid or wrong-room token                                                      |
| 404    | Room not found                                                                            |
| 409    | Room closed                                                                               |
| 413    | Request body over 4 KB                                                                    |
| 500    | Unexpected error. The real error is logged and the client gets a generic `internal error` |

## How it works

**Tokens.** `enter` increments `next_number` atomically and returns an HMAC-SHA256 signed token `{room_id, number, issued_at}`. `status` verifies it locally with no database call. All instances must share `QUEUE_SECRET`.

**Atomic counters.** `NextNumber` and `Admit` are each a single `UPDATE ... RETURNING` statement. `Admit` is capped with `MIN(admitted_up_to + n, next_number)` and only applies to open rooms.

**Status never queries the DB per poll.** `roomcache` holds the latest `RoomState` per room behind a `sync.RWMutex`. It fills on first request and a ticker goroutine refreshes it every second. Known gap: an unknown room id hits Turso on every request (add a short negative cache later).

**Admitter.** Every second it reads a snapshot, skips paused or closed rooms and a rate of 0, accumulates fractional "people credit" from the elapsed time, and admits `floor(credit)` capped at the number waiting. If `Admit` fails, the credit is kept and retried next tick. It stops on context cancellation.

**Errors map to status codes in one place** (`writeError`), via sentinel errors and `errors.Is`.

## Roadmap

1. Admin API: `Reset`, then endpoints for rate, pause/resume, admit N and reset. Decide the reset-token behaviour (token epoch vs. invalidate on reset).
2. Redis `CounterStore` (`INCR`, atomic capped `Admit` via Lua), hydrate from Turso on boot, periodic flush to Turso, `COUNTER_STORE=turso|redis` flag.
3. Admitter leader lock (`SET NX PX` with renewal) and a two-instance drill.
4. Pass tokens, `POST /verify`, then the React waiting page in `frontend/`.
5. Optional: Turso store tests (50-goroutine `NextNumber` test), load test comparing Turso vs. Redis.
   The full plan lives in the project doc `queue-app-plan.md`.
