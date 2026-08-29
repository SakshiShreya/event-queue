package queue

import (
	"fmt"
	"sync"
	"time"
)

// Ticket statuses
const (
	StatusWaiting = "waiting"
	StatusCalled  = "called"
	StatusSkipped = "skipped"
	StatusDone    = "done"
)

// Ticket represents a person in the queue
type Ticket struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
	Status    string `json:"status"`
	JoinedAt  int64  `json:"joined_at"`
	PartySize int    `json:"party_size"`

	slot int // permanent 1-based Fenwick index, assigned at Join
}

// Queue represents a single queue
type Queue struct {
	mu      sync.RWMutex
	tickets []*Ticket
	byID    map[string]*Ticket // O(1) lookup instead of a linear scan
	waiting *Fenwick           // 1 per waiting slot, 0 otherwise
	counter int
}

func New() *Queue {
	return &Queue{
		tickets: []*Ticket{},
		byID:    make(map[string]*Ticket),
		waiting: NewFenwick(1024),
	}
}

var defaultQueue = New()

// GetDefault returns singleton queue instance
func GetDefault() *Queue {
	return defaultQueue
}

// Join adds a new ticket to the queue
func (q *Queue) Join(name string, partySize int) (*Ticket, error) {
	// Validate
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	if partySize <= 0 {
		return nil, fmt.Errorf("party_size must be > 0")
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// Generate ID
	q.counter++
	slot := q.counter
	id := fmt.Sprintf("ticket-%03d", slot)

	// create ticket
	ticket := &Ticket{
		ID:        id,
		Name:      name,
		Status:    StatusWaiting,
		JoinedAt:  time.Now().Unix(),
		PartySize: partySize,
		slot:      slot,
	}

	q.tickets = append(q.tickets, ticket)
	q.byID[ticket.ID] = ticket
	q.waiting.Add(slot, 1)

	return q.snapshot(ticket), nil
}

// Position returns the live 1-based position of a waiting ticket, or 0 if the
// ticket has already left the line. O(log n).
func (q *Queue) Position(ticketID string) (int, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	t, ok := q.byID[ticketID]
	if !ok {
		return 0, fmt.Errorf("ticket not found")
	}
	if t.Status != StatusWaiting {
		return 0, nil
	}
	// Everyone still waiting at or before this slot
	return q.waiting.PrefixSum(t.slot), nil
}

// Get returns a single ticket with its position filled in. O(log n).
func (q *Queue) Get(ticketId string) (*Ticket, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	t, ok := q.byID[ticketId]
	if !ok {
		return nil, fmt.Errorf("ticket not found")
	}
	return q.snapshot(t), nil
}

// WaitingCount returns how many people are still in line. O(log n).
func (q *Queue) WaitingCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.waiting.Total()
}

// GetAll returns all tickets
func (q *Queue) GetAll() []Ticket {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// return a copy to avoid race conditions
	result := make([]Ticket, 0, len(q.tickets))
	pos := 0
	for _, t := range q.tickets {
		c := *t
		if t.Status == StatusWaiting {
			pos++
			c.Position = pos
		}
		result = append(result, c)
	}
	return result
}

// Call marks the first ticket as called
func (q *Queue) Call() (*Ticket, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// The first waiting ticket is the 1st set slot — no scanning.
	slot := q.waiting.FindKth(1)
	if slot == 0 {
		return nil, fmt.Errorf("no waiting tickets")
	}

	t := q.tickets[slot-1]
	q.leaveline(t)
	t.Status = StatusCalled
	return q.snapshot(t), nil
}

// Skip marks a ticket as skipped
func (q *Queue) Skip(ticketId string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	t, ok := q.byID[ticketId]
	if !ok {
		return fmt.Errorf("ticket not found")
	}
	if t.Status != StatusWaiting && t.Status != StatusCalled {
		return fmt.Errorf("can only skip waiting or called tickets")
	}

	q.leaveline(t)
	t.Status = StatusSkipped
	return nil
}

// Serve marks a ticket as "done"
func (q *Queue) Serve(ticketId string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	t, ok := q.byID[ticketId]
	if !ok {
		return fmt.Errorf("ticket not found")
	}
	if t.Status != StatusCalled {
		return fmt.Errorf("only called tickets can be served")
	}

	q.leaveline(t) // already cleared by Call, but keep the transition self-contained
	t.Status = StatusDone
	return nil
}

func (q *Queue) leaveline(t *Ticket) {
	if t.Status == StatusWaiting {
		q.waiting.Add(t.slot, -1)
	}
}

// snapshot copies a ticket and fills in its live position.
// Caller must hold at least the read lock.
func (q *Queue) snapshot(t *Ticket) *Ticket {
	c := *t
	c.Position = 0
	if t.Status == StatusWaiting {
		c.Position = q.waiting.PrefixSum(t.slot)
	}
	return &c
}
