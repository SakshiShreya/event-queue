package main

import (
	"math/rand/v2"
	"net/http"
	"queue-app/internal/roomcache"
	"queue-app/internal/store"
	"queue-app/internal/token"
	"time"

	"github.com/go-chi/chi/v5"
)

const retryAfterSeconds = 5

type server struct {
	counters store.CounterStore
	cache    *roomcache.Cache
	signer   *token.Signer
}

type enterResponse struct {
	Token      string `json:"token"`
	Number     int64  `json:"number"`
	RetryAfter int    `json:"retryAfter"`
}

type statusResponse struct {
	State            string `json:"state"`
	Ahead            int64  `json:"ahead"`
	RetryAfter       int    `json:"retryAfter"`
	EstimatedWaitSec int64  `json:"estimatedWaitSec"`
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

func (s *server) status(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")

	tok := r.Header.Get("X-Queue-Token")
	if tok == "" {
		writeError(w, store.ErrUnauthorized)
		return
	}
	p, err := s.signer.Verify(tok)
	if err != nil || p.RoomID != roomID {
		writeError(w, store.ErrUnauthorized)
		return
	}

	state, err := s.cache.Get(r.Context(), roomID)
	if err != nil {
		writeError(w, err)
		return
	}

	ahead := p.Number - state.AdmittedUpTo
	if ahead < 0 {
		ahead = 0
	}

	var st string
	switch {
	case state.Status == store.StatusClosed:
		st = "closed"
	case state.Status == store.StatusPaused:
		st = "paused"
	case ahead == 0:
		st = "admitted"
	default:
		st = "waiting"
	}

	eta := int64(-1)
	if state.AdmitRatePerMin > 0 {
		eta = ahead * 60 / state.AdmitRatePerMin
	}

	retryAfter := retryAfterSeconds + rand.IntN(3)

	writeJSON(w, statusResponse{
		State:            st,
		Ahead:            ahead,
		RetryAfter:       retryAfter,
		EstimatedWaitSec: eta,
	})
}
