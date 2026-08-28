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
	mux.HandleFunc("POST /call", callHandler)
	mux.HandleFunc("POST /skip", skipHandler)

	// start server
	port := ":8080"
	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(port, mux))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok"}`)
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}

func queueHandler(w http.ResponseWriter, r *http.Request) {
	tickets := q.GetAll()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tickets": tickets,
		"count":   len(tickets),
	})
}

func callHandler(w http.ResponseWriter, r *http.Request) {
	ticket, err := q.Call()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ticket)
}

func skipHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TicketID string `json:"ticket_id"`
	}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	err = q.Skip(req.TicketID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"success":true}`)
}
