package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"queue-app/internal/db"
	"queue-app/internal/queue"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

var q queue.Store

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("no .env file loaded: %v", err)
	}

	r := chi.NewRouter()

	r.Use(withLogging)
	r.Use(chimiddleware.Recoverer)
	r.Use(cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:5173",
		},
		AllowedMethods: []string{http.MethodGet, http.MethodPost},
		AllowedHeaders: []string{"Content-Type"},
	}).Handler)
	r.Use(limitBody(4 << 10)) // 4 KB

	// endpoints
	r.Get("/health", healthHandler)

	r.Post("/join", joinHandler)
	r.Get("/queue", queueHandler)
	r.Get("/tickets/{id}", ticketHandler)
	r.Post("/call", callHandler)
	r.Post("/start", startHandler)
	r.Post("/skip", skipHandler)
	r.Post("/done", doneHandler)

	sqlDB, err := db.Open(os.Getenv("TURSO_DATABASE_URL"), os.Getenv("TURSO_AUTH_TOKEN"))
	if err != nil {
		log.Fatalf("failed to connect to turso: %v", err)
	}
	defer sqlDB.Close()
	log.Println("connected to turso")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.Migrate(ctx, sqlDB); err != nil {
		log.Fatalf("migrate failed: %v", err)
	}
	log.Println("migrated")

	q = queue.NewDBStore(sqlDB, "default")

	// start server
	port := ":8080"
	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, r))
}

// statusRecorder wraps http.ResponseWriter to capture the status code that
// gets written, since ResponseWriter itself doesn't expose it after the fact.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// withLogging wraps a handler, logging method, path, status, and duration
// for every request. It's placed outermost (wrapping the CORS handler) so
// it logs every request, including rejected preflights.
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// default to 200: if the handler never calls WriteHeader explicitly
		// (e.g. it just writes a body), net/http implicitly sends 200 too.
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)
		log.Printf("%s %s %d %v", r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

type badRequest struct{ msg string }

func (e badRequest) Error() string { return e.msg }

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError

	switch {
	case errors.Is(err, queue.ErrValidation):
		status = http.StatusBadRequest
	case errors.Is(err, queue.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, queue.ErrConflict):
		status = http.StatusConflict
	default:
		var br badRequest
		if errors.As(err, &br) {
			status = http.StatusBadRequest
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func joinHandler(w http.ResponseWriter, r *http.Request) {
	// parse json from request body
	var req struct {
		Name      string `json:"name"`
		PartySize *int   `json:"party_size"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, badRequest{"Invalid JSON"})
		return
	}

	partySize := 1
	if req.PartySize != nil && *req.PartySize != 0 {
		partySize = *req.PartySize
	}
	// add to queue
	ticket, err := q.Join(r.Context(), req.Name, partySize)
	if err != nil {
		writeError(w, err)
		return
	}

	// return the ticket
	writeJSON(w, ticket)
}

func queueHandler(w http.ResponseWriter, r *http.Request) {
	tickets, err := q.GetAll(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{
		"tickets": tickets,
		"count":   len(tickets),
	})
}

func ticketHandler(w http.ResponseWriter, r *http.Request) {
	ticket, err := q.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}

	waitingCount, err := q.WaitingCount(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, map[string]any{
		"ticket":        ticket,
		"position":      ticket.Position,
		"waiting_count": waitingCount,
	})
}

func callHandler(w http.ResponseWriter, r *http.Request) {
	ticket, err := q.Call(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, ticket)
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	takeAction(w, r, q.Start)
}

func skipHandler(w http.ResponseWriter, r *http.Request) {
	takeAction(w, r, q.Skip)
}

func doneHandler(w http.ResponseWriter, r *http.Request) {
	takeAction(w, r, q.Done)
}

func takeAction(w http.ResponseWriter, r *http.Request, do func(context.Context, string) error) {
	var req struct {
		TicketID string `json:"ticket_id"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		writeError(w, badRequest{"Invalid JSON"})
		return
	}
	if req.TicketID == "" {
		writeError(w, badRequest{"ticket_id is required"})
		return
	}

	err = do(r.Context(), req.TicketID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, map[string]bool{"success": true})
}

// limitBody caps how many bytes a handler can read from the request body.
// Reads past the limit fail with *http.MaxBytesError.
func limitBody(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}
