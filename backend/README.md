# Backend — Go HTTP Server

Queue management API with Turso database backend.

## Setup

```bash
go run ./cmd/server
```

Server starts on `http://localhost:8080`.

## Endpoints

- `GET /health` — health check
- `POST /join` — join queue
- `GET /queue` — get queue state
- `POST /call` — call next person
- `POST /skip` — skip person

Test:

```bash
curl http://localhost:8080/health
```
