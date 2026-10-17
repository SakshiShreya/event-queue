package roomcache

import (
	"context"
	"log"
	"queue-app/internal/store"
	"sync"
	"time"
)

type Cache struct {
	counters   store.CounterStore
	roomStates map[string]store.RoomState
	mu         sync.RWMutex
}

func New(counters store.CounterStore) *Cache {
	return &Cache{
		counters:   counters,
		roomStates: make(map[string]store.RoomState),
	}
}

func (c *Cache) Get(ctx context.Context, roomID string) (store.RoomState, error) {
	c.mu.RLock()
	roomState, exists := c.roomStates[roomID]
	c.mu.RUnlock()

	if exists {
		return roomState, nil
	}

	roomState, err := c.counters.Snapshot(ctx, roomID)
	if err != nil {
		return store.RoomState{}, err
	}

	c.mu.Lock()
	c.roomStates[roomID] = roomState
	c.mu.Unlock()

	return roomState, nil
}

func (c *Cache) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.refresh(ctx)
		}
	}
}

func (c *Cache) refresh(ctx context.Context) {
	c.mu.RLock()
	roomIDs := make([]string, 0, len(c.roomStates))
	for id := range c.roomStates {
		roomIDs = append(roomIDs, id)
	}
	c.mu.RUnlock()

	for _, roomID := range roomIDs {
		roomState, err := c.counters.Snapshot(ctx, roomID)
		if err != nil {
			log.Println(err)
		}

		c.mu.Lock()
		c.roomStates[roomID] = roomState
		c.mu.Unlock()
	}
}
