# Event Queueing

Event/waitlist queue system: people join a line, admin calls them in order.

**Stack:** React (frontend) · Go (backend) · Turso SQLite (database) · Polling (no WebSockets)

## Quick Start

### Backend Setup

```bash
cd backend
go run ./cmd/server
```

Server: `http://localhost:8080`

### Frontend Setup

```bash
cd frontend
npm install
npm run dev
```

App: `http://localhost:5173`

## Tech Stack

**Backend:**

- Go 1.21+
- `net/http` standard library
- Turso (cloud SQLite)
- `libsql-client-go` for database

**Frontend:**

- React 18+
- Vite (bundler)
- Polling (3–5s intervals)
- No WebSockets (simplicity first)

## License

MIT — see LICENSE file.

## Getting Help

- Go syntax: [Tour of Go](https://tour.golang.org) + [Go by Example](https://gobyexample.com)
- React: [React docs](https://react.dev)
- Turso: [Turso docs](https://docs.turso.tech)
