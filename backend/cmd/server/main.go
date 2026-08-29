package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"queue-app/internal/queue"
)

var q = queue.GetDefault()

func main() {
	mux := http.NewServeMux()

	// endpoints
	mux.HandleFunc("GET /health", healthHandler)

	mux.HandleFunc("POST /join", joinHandler)
	mux.HandleFunc("GET /queue", queueHandler)
	mux.HandleFunc("GET /tickets/{id}", ticketHandler)
	mux.HandleFunc("POST /call", callHandler)
	mux.HandleFunc("POST /skip", skipHandler)
	mux.HandleFunc("POST /serve", serveHandler)

	// start server
	port := ":8080"
	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok"})
}

func joinHandler(w http.ResponseWriter, r *http.Request) {
	// parse json from request body
	var req struct {
		Name      string `json:"name"`
		PartySize int    `json:"party_size"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// add to queue
	ticket, err := q.Join(req.Name, req.PartySize)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// return the ticket
	writeJSON(w, ticket)
}

func queueHandler(w http.ResponseWriter, r *http.Request) {
	tickets := q.GetAll()
	writeJSON(w, map[string]interface{}{
		"tickets": tickets,
		"count":   len(tickets),
	})
}

func ticketHandler(w http.ResponseWriter, r *http.Request) {
	ticket, err := q.Get(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]any{
		"ticket":        ticket,
		"position":      ticket.Position,
		"waiting_count": q.WaitingCount(),
	})
}

func callHandler(w http.ResponseWriter, r *http.Request) {
	ticket, err := q.Call()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, ticket)
}

func skipHandler(w http.ResponseWriter, r *http.Request) {
	takeAction(w, r, q.Skip)
}

func serveHandler(w http.ResponseWriter, r *http.Request) {
	takeAction(w, r, q.Serve)
}

func takeAction(w http.ResponseWriter, r *http.Request, do func(string) error) {
	var req struct {
		TicketID string `json:"ticket_id"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err = do(req.TicketID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]bool{"success": true})
}
