# Backend — Go HTTP Server

Queue management API with Turso database backend.

## Setup

```bash
go run ./cmd/server
```

Server starts on `http://localhost:8080`.

## Endpoints

| Method | Path            | Body                               | Description                                                |
| ------ | --------------- | ---------------------------------- | ---------------------------------------------------------- |
| GET    | `/health`       | —                                  | Health check                                               |
| POST   | `/join`         | `{"name": "...", "party_size": 1}` | Join the queue, returns the new ticket                     |
| GET    | `/queue`        | —                                  | Full ticket list + count                                   |
| GET    | `/tickets/{id}` | —                                  | Single ticket, its live position, and how many are waiting |
| POST   | `/call`         | —                                  | Calls the next waiting ticket (lowest position)            |
| POST   | `/skip`         | `{"ticket_id": "..."}`             | Skips a waiting or called ticket                           |
| POST   | `/serve`        | `{"ticket_id": "..."}`             | Marks a called ticket as done                              |

Ticket statuses: `waiting` → `called` → (`done` or `skipped`). A `waiting` ticket can also be skipped directly.

Test:

```bash
curl http://localhost:8080/health
```

## Position tracking

A ticket's `position` isn't stored — it's the count of everyone still `waiting` at or before that ticket's join order, computed live on every read. That's backed by a **Fenwick tree** (`internal/queue/fenwick.go`), not a plain loop:

- Each ticket gets a fixed slot = its join order (1-based).
- The tree holds 1 for slots still `waiting`, 0 otherwise.
- `Position` / `PrefixSum` — O(log n)
- Leaving the line (`Call`, `Skip`, `Serve`) — O(log n) update, not a shift of everyone behind
- `Call()` finds the next waiting ticket via `FindKth(1)` (the first set slot) — O(log n), no scanning
- Capacity grows by doubling + full rebuild when needed; amortized O(log n) per op even across growth
  This is overkill for the actual scale of this app (tens of people, one process) — it was built as a deliberate learning exercise for week 3, not because plain O(n) recalculation on read wasn't good enough. See `internal/queue/verify_test.go`-style tests for the growth/correctness stress tests if you're picking this back up later and want to trust it again.

## Concurrency

`Queue` uses a `sync.RWMutex`: reads (`Get`, `Position`, `WaitingCount`, `GetAll`) take a read lock; mutations (`Join`, `Call`, `Skip`, `Serve`) take a write lock.
