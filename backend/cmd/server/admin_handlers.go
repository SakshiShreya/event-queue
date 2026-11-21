package main

import (
	"context"
	"encoding/json"
	"net/http"
	"queue-app/internal/store"

	"github.com/go-chi/chi/v5"
)

type snapshotResponse struct {
	Status          string `json:"status"`
	AdmitRatePerMin int64  `json:"admitRatePerMin"`
	NextNumber      int64  `json:"nextNumber"`
	AdmittedUpTo    int64  `json:"admittedUpTo"`
}

type rateRequest struct {
	PerMin int64 `json:"perMin"`
}

type admitRequest struct {
	N int64 `json:"n"`
}

func (s *server) writeSnapshot(ctx context.Context, w http.ResponseWriter, roomID string) {
	roomState, err := s.counters.Snapshot(ctx, roomID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, snapshotResponse{Status: string(roomState.Status), AdmitRatePerMin: roomState.AdmitRatePerMin, NextNumber: roomState.NextNumber, AdmittedUpTo: roomState.AdmittedUpTo})
}

func (s *server) snapshot(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")

	s.writeSnapshot(r.Context(), w, roomID)
}

func (s *server) rate(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")
	var req rateRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, badRequest{"invalid JSON body"})
		return
	}

	if req.PerMin <= 0 {
		writeError(w, badRequest{"perMin must be greater than 0"})
		return
	}

	if err := s.counters.SetRate(r.Context(), roomID, req.PerMin); err != nil {
		writeError(w, err)
		return
	}

	s.writeSnapshot(r.Context(), w, roomID)
}

func (s *server) admit(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")
	var req admitRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, badRequest{"invalid JSON body"})
		return
	}

	if req.N <= 0 {
		writeError(w, badRequest{"n must be greater than 0"})
		return
	}

	roomState, err := s.counters.Snapshot(r.Context(), roomID)
	if err != nil {
		writeError(w, err)
		return
	}
	if roomState.Status == store.StatusClosed {
		writeError(w, store.ErrRoomClosed)
		return
	}
	if roomState.Status != store.StatusOpen {
		writeError(w, store.ErrRoomNotOpen)
		return
	}

	if _, err := s.counters.Admit(r.Context(), roomID, req.N); err != nil {
		writeError(w, err)
		return
	}

	s.writeSnapshot(r.Context(), w, roomID)
}

func (s *server) setStatus(ctx context.Context, w http.ResponseWriter, roomID string, status store.Status) {
	if err := s.counters.SetStatus(ctx, roomID, status); err != nil {
		writeError(w, err)
		return
	}

	s.writeSnapshot(ctx, w, roomID)
}

func (s *server) pause(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")

	roomState, err := s.counters.Snapshot(r.Context(), roomID)
	if err != nil {
		writeError(w, err)
		return
	}
	if roomState.Status == store.StatusClosed {
		writeError(w, store.ErrRoomClosed)
		return
	}

	s.setStatus(r.Context(), w, roomID, store.StatusPaused)
}

func (s *server) open(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")

	s.setStatus(r.Context(), w, roomID, store.StatusOpen)
}

func (s *server) close(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "id")

	s.setStatus(r.Context(), w, roomID, store.StatusClosed)
}
