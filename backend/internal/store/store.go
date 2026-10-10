package store

import (
	"context"
	"time"
)

type CounterStore interface {
	NextNumber(ctx context.Context, roomID string) (int64, error) // This method is atomic, returns the new number, or ErrNotFound/ErrRoomClosed
	Snapshot(ctx context.Context, roomID string) (RoomState, error)
	Admit(ctx context.Context, roomID string, n int64) (int64, error) // returns new admitted_up_to value, never exceeds next_number
	// SetStatus(ctx context.Context, roomID string) error
	// SetRate(ctx context.Context, roomID string) error
	// Reset(ctx context.Context, roomID string) error
	AcquireAdmitterLock(ctx context.Context, roomID string, ttl time.Duration) (bool, error)
}

type RoomRepo interface {
	LoadRoom(ctx context.Context, roomID string) (Room, error)
	SaveRoom(ctx context.Context, room Room) error
	SaveCounters(ctx context.Context, roomID string, nextNumber int64, admittedUpTo int64) error
}
