package main

import (
	"net/http"
	"queue-app/internal/store"
	"queue-app/internal/token"
	"time"

	"github.com/go-chi/chi/v5"
)

const retryAfterSeconds = 5

type server struct {
	counters store.CounterStore
	signer   *token.Signer
}

type enterResponse struct {
	Token      string `json:"token"`
	Number     int64  `json:"number"`
	RetryAfter int    `json:"retryAfter"`
}

func (s *server) enter(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")

	// Already in line? A valid token for THIS room returns the same number.
	// A missing, invalid or other-room token is treated as a new visitor.
	if tok := r.Header.Get("X-Queue-Token"); tok != "" {
		p, err := s.signer.Verify(tok)
		if err == nil && p.RoomID == roomID {
			writeJSON(w, enterResponse{Token: tok, Number: p.Number, RetryAfter: retryAfterSeconds})
			return
		}
	}

	n, err := s.counters.NextNumber(r.Context(), roomID)
	if err != nil {
		writeError(w, err)
		return
	}

	tok, err := s.signer.Sign(token.Payload{RoomID: roomID, Number: n, IssuedAt: time.Now().Unix()})
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, enterResponse{Token: tok, Number: n, RetryAfter: retryAfterSeconds})
}
