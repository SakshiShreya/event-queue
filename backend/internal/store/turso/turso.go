package turso

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"queue-app/internal/store"
	"time"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store {
	return &Store{db}
}

func (s *Store) AcquireAdmitterLock(ctx context.Context, roomID string, ttl time.Duration) (bool, error) {
	return true, nil
}

func (s *Store) Admit(ctx context.Context, roomID string, n int64) (int64, error) {
	if n < 0 {
		return 0, fmt.Errorf("admit n must not be negative: %w", store.ErrValidation)
	}

	var admittedUpTo int64
	err := s.db.QueryRowContext(ctx,
		`UPDATE Rooms SET admitted_up_to = MIN(admitted_up_to + ?, next_number)
		 WHERE id = ? AND status = 'open'
		 RETURNING admitted_up_to`, n, roomID).Scan(&admittedUpTo)
	if err == nil {
		return admittedUpTo, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("admit for room %q: %w", roomID, err)
	}

	// Nothing was updated: the room is missing, paused or closed. Find out which.
	roomState, err := s.Snapshot(ctx, roomID)
	if err != nil {
		return 0, err
	}
	return roomState.AdmittedUpTo, nil
}

func (s *Store) NextNumber(ctx context.Context, roomID string) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`UPDATE Rooms SET next_number = next_number + 1
		 WHERE id = ? AND status != 'closed'
		 RETURNING next_number`, roomID).Scan(&n)
	if err == nil {
		return n, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("next number for room %q: %w", roomID, err)
	}

	// Nothing was updated: the room is missing or closed. Find out which.
	var status store.Status
	err = s.db.QueryRowContext(ctx,
		`SELECT status FROM Rooms WHERE id = ?`, roomID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, store.ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("lookup room %q: %w", roomID, err)
	}
	if status == store.StatusClosed {
		return 0, store.ErrRoomClosed
	}
	return 0, fmt.Errorf("room %q changed state during enter, retry", roomID)
}

func (s *Store) Snapshot(ctx context.Context, roomID string) (store.RoomState, error) {
	var status store.Status
	var admitRatePerMin, nextNumber, admittedUpTo int64
	err := s.db.QueryRowContext(ctx,
		"SELECT status, admit_rate_per_min, next_number, admitted_up_to FROM Rooms WHERE id = ?",
		roomID).Scan(&status, &admitRatePerMin, &nextNumber, &admittedUpTo)
	if errors.Is(err, sql.ErrNoRows) {
		return store.RoomState{}, store.ErrNotFound
	}
	if err != nil {
		return store.RoomState{}, fmt.Errorf("lookup room %q: %w", roomID, err)
	}

	roomState := store.RoomState{
		Status:          status,
		AdmitRatePerMin: admitRatePerMin,
		NextNumber:      nextNumber,
		AdmittedUpTo:    admittedUpTo,
	}
	return roomState, nil
}

func (s *Store) LoadRoom(ctx context.Context, roomID string) (store.Room, error) {
	room := store.Room{}
	return room, nil
}

func (s *Store) SaveRoom(ctx context.Context, room store.Room) error {
	return nil
}

func (s *Store) SaveCounters(ctx context.Context, roomID string, nextNumber int64, admittedUpTo int64) error {
	return nil
}

var _ store.CounterStore = (*Store)(nil)
var _ store.RoomRepo = (*Store)(nil)
