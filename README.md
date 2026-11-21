# Event Queueing — Virtual Waiting Room

A virtual waiting room that protects a site from traffic spikes. Visitors get a signed ticket number, poll for their position, and are admitted at a configurable rate.

**Stack:** Go (API) · Turso (durable config and snapshots) · Redis (hot counters, planned) · React + Vite (waiting page, planned) · polling, no WebSockets

## Quick start

### Backend Setup

```bash
cd backend
# create backend/.env first (see backend/README.md)
go run ./cmd/server
```

Server: `http://localhost:8080`

### Frontend Setup

```bash
cd frontend
npm install
npm run dev     # http://localhost:5173
```

## How it works (short version)

- `enter` atomically increments a counter and returns an HMAC-signed token containing the visitor's number. There is no per-visitor row.
- `status` verifies the token locally and computes `ahead = number - admitted_up_to` from a cache that refreshes every second.
- A background admitter advances `admitted_up_to` at the room's rate.

## Layout

```
backend/     Go API
frontend/    React waiting page (not started)
references/  Learning leftovers (Fenwick tree)
```
