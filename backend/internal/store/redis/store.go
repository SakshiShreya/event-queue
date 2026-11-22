package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"queue-app/internal/store"

	goredis "github.com/redis/go-redis/v9"
)

var errNotImplemented = errors.New("not implemented")

type Store struct {
	client *goredis.Client
}

func New(client *goredis.Client) *Store {
	return &Store{client: client}
}

func statusKey(roomID string) string     { return "room:" + roomID + ":status" }
func nextNumberKey(roomID string) string { return "room:" + roomID + ":next_number" }

// nextNumberScript runs atomically inside Redis.
// KEYS[1] = status key, KEYS[2] = next_number key.
// Returns -1 if the room doesn't exist, -2 if it is closed,
// otherwise the new ticket number.
var nextNumberScript = goredis.NewScript(`
local status = redis.call('GET', KEYS[1])
if not status then
	return -1
end
if status == 'closed' then
	return -2
end
return redis.call('INCR', KEYS[2])
`)

func (s *Store) NextNumber(ctx context.Context, roomID string) (int64, error) {
	n, err := nextNumberScript.Run(ctx, s.client,
		[]string{statusKey(roomID), nextNumberKey(roomID)}).Int64()
	if err != nil {
		return 0, fmt.Errorf("next number for room %q: %w", roomID, err)
	}

	switch n {
	case -1:
		return 0, store.ErrNotFound
	case -2:
		return 0, store.ErrRoomClosed
	}
	return n, nil
}

// The methods below are filled in later (Snapshot next, the rest in Week 12).

func (s *Store) Snapshot(ctx context.Context, roomID string) (store.RoomState, error) {
	return store.RoomState{}, errNotImplemented
}

func (s *Store) Admit(ctx context.Context, roomID string, n int64) (int64, error) {
	return 0, errNotImplemented
}

func (s *Store) SetStatus(ctx context.Context, roomID string, status store.Status) error {
	return errNotImplemented
}

func (s *Store) SetRate(ctx context.Context, roomID string, admitRatePerMin int64) error {
	return errNotImplemented
}

func (s *Store) AcquireAdmitterLock(ctx context.Context, roomID string, ttl time.Duration) (bool, error) {
	return false, errNotImplemented
}

var _ store.CounterStore = (*Store)(nil)
