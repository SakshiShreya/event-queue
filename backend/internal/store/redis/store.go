package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"queue-app/internal/store"

	goredis "github.com/redis/go-redis/v9"
)

type Store struct {
	client *goredis.Client
}

func New(client *goredis.Client) *Store {
	return &Store{client: client}
}

func statusKey(roomID string) string       { return "room:" + roomID + ":status" }
func nextNumberKey(roomID string) string   { return "room:" + roomID + ":next_number" }
func rateKey(roomID string) string         { return "room:" + roomID + ":rate" }
func admittedUpToKey(roomID string) string { return "room:" + roomID + ":admitted_up_to" }

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

// KEYS: [1] is statusKey, [2] is nextNumberKey, [3] is admittedUpToKey
// ARGV: [1] is n, which you pass from Go
// Returns: -1 if the room doesn't exist, which maps to ErrNotFound. -2 if the room is paused or closed,
// which maps to ErrRoomNotOpen. Otherwise the new admitted_up_to
var admitScript = goredis.NewScript(`
local status = redis.call('GET', KEYS[1])
if not status then
	return -1
end
if status ~= 'open' then
	return -2
end

local n = tonumber(ARGV[1])
local nextNumber = tonumber(redis.call('GET', KEYS[2]) or '0')
local admitted = tonumber(redis.call('GET', KEYS[3]) or '0')

local newAdmitted = math.max(admitted, math.min(admitted + n, nextNumber))
redis.call('SET', KEYS[3], newAdmitted)
return newAdmitted
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

func (s *Store) Snapshot(ctx context.Context, roomID string) (store.RoomState, error) {
	vals, err := s.client.MGet(ctx,
		statusKey(roomID),
		rateKey(roomID),
		nextNumberKey(roomID),
		admittedUpToKey(roomID),
	).Result()
	if err != nil {
		return store.RoomState{}, fmt.Errorf("snapshot for room %q: %w", roomID, err)
	}

	// vals[0] is the status; a missing status key means the room doesn't exist
	statusStr, ok := vals[0].(string)
	if !ok {
		return store.RoomState{}, store.ErrNotFound
	}
	status := store.Status(statusStr)
	switch status {
	case store.StatusOpen, store.StatusPaused, store.StatusClosed:
	default:
		return store.RoomState{}, fmt.Errorf("room %q has unknown status %q", roomID, statusStr)
	}

	rate, err := toInt64(vals[1])
	if err != nil {
		return store.RoomState{}, fmt.Errorf("room %q rate: %w", roomID, err)
	}
	nextNumber, err := toInt64(vals[2])
	if err != nil {
		return store.RoomState{}, fmt.Errorf("room %q next_number: %w", roomID, err)
	}
	admittedUpTo, err := toInt64(vals[3])
	if err != nil {
		return store.RoomState{}, fmt.Errorf("room %q admitted_up_to: %w", roomID, err)
	}

	return store.RoomState{
		Status:          status,
		AdmitRatePerMin: rate,
		NextNumber:      nextNumber,
		AdmittedUpTo:    admittedUpTo,
	}, nil
}

// toInt64 converts an MGET value: nil (key missing) means 0, a string is parsed.
func toInt64(v any) (int64, error) {
	if v == nil {
		return 0, nil
	}
	str, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("unexpected type %T", v)
	}
	return strconv.ParseInt(str, 10, 64)
}

func (s *Store) Admit(ctx context.Context, roomID string, n int64) (int64, error) {
	if n <= 0 {
		return 0, store.ErrValidation
	}
	newAdmitted, err := admitScript.Run(ctx, s.client,
		[]string{statusKey(roomID), nextNumberKey(roomID), admittedUpToKey(roomID)}, n).Int64()
	if err != nil {
		return 0, fmt.Errorf("next number for room %q: %w", roomID, err)
	}

	switch newAdmitted {
	case -1:
		return 0, store.ErrNotFound
	case -2:
		return 0, store.ErrRoomNotOpen
	}
	return newAdmitted, nil
}

func (s *Store) SetStatus(ctx context.Context, roomID string, status store.Status) error {
	ok, err := s.client.SetXX(ctx, statusKey(roomID), string(status), 0).Result()

	if err != nil {
		return err
	}

	if !ok {
		return store.ErrNotFound
	}

	return nil
}

func (s *Store) SetRate(ctx context.Context, roomID string, admitRatePerMin int64) error {
	ok, err := s.client.SetXX(ctx, rateKey(roomID), admitRatePerMin, 0).Result()

	if err != nil {
		return err
	}

	if !ok {
		return store.ErrNotFound
	}

	return nil
}

func (s *Store) AcquireAdmitterLock(ctx context.Context, roomID string, ttl time.Duration) (bool, error) {
	var errNotImplemented = errors.New("not implemented")
	return false, errNotImplemented
}

var _ store.CounterStore = (*Store)(nil)
