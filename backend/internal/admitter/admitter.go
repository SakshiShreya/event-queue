package admitter

import (
	"context"
	"log"
	"math"
	"queue-app/internal/store"
	"time"
)

type Admitter struct {
	counters store.CounterStore
	roomID   string
}

func New(counters store.CounterStore, roomID string) *Admitter {
	return &Admitter{
		counters: counters,
		roomID:   roomID,
	}
}

func (a *Admitter) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	leftoverPeopleCredit := 0.0
	var lastTick time.Time

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			roomState, err := a.counters.Snapshot(ctx, a.roomID)
			if err != nil {
				log.Println(err)
				leftoverPeopleCredit, lastTick = 0, time.Time{}
				continue
			}

			waiting := roomState.NextNumber - roomState.AdmittedUpTo
			if roomState.Status != store.StatusOpen || roomState.AdmitRatePerMin <= 0 || waiting <= 0 {
				leftoverPeopleCredit, lastTick = 0, time.Time{}
				continue
			}

			now := time.Now()

			if lastTick.IsZero() {
				lastTick = now
				continue
			}

			secondsElapsed := now.Sub(lastTick).Seconds()
			lastTick = now
			leftoverPeopleCredit += float64(roomState.AdmitRatePerMin) * secondsElapsed / 60

			n := int64(math.Floor(leftoverPeopleCredit))
			if n <= 0 {
				continue
			}
			n = min(n, waiting)

			admittedUpTo, err := a.counters.Admit(ctx, a.roomID, n)
			if err != nil {
				log.Println(err) // keep the credit and retry next tick
				continue
			}
			leftoverPeopleCredit -= float64(n)
			log.Printf("admitted %d, admitted_up_to=%d", n, admittedUpTo)
		}
	}
}
