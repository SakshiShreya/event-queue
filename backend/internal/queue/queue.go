package queue

import (
	"fmt"
	"sync"
	"time"
)

// Ticket represents a person in the queue
type Ticket struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Position  int    `json:"position"`
	Status    string `json:"status"`
	JoinedAt  int64  `json:"joined_at"`
	PartySize int    `json:"party_size"`
}

// Queue represents a single queue
type Queue struct {
	mu      sync.Mutex
	tickets []Ticket
	counter int
}

var defaultQueue = &Queue{
	tickets: []Ticket{},
	counter: 0,
}

// Join adds a new ticket to the queue
func (q *Queue) Join(name string, partySize int) (*Ticket, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Validate
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	if partySize <= 0 {
		return nil, fmt.Errorf("party_size must be > 0")
	}

	// Generate ID
	q.counter++
	id := fmt.Sprintf("ticket-%03d", q.counter)

	// create ticket
	ticket := Ticket{
		ID:        id,
		Name:      name,
		Position:  len(q.tickets) + 1,
		Status:    "waiting",
		JoinedAt:  time.Now().Unix(),
		PartySize: partySize,
	}

	q.tickets = append(q.tickets, ticket)
	return &ticket, nil
}

// GetAll returns all tickets
func (q *Queue) GetAll() []Ticket {
	q.mu.Lock()
	defer q.mu.Unlock()

	// return a copy to avoid race conditions
	result := make([]Ticket, len(q.tickets))
	copy(result, q.tickets)
	return result
}

// Call marks the first ticket as called
func (q *Queue) Call() (*Ticket, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, t := range q.tickets {
		if t.Status == "waiting" {
			q.tickets[i].Status = "called"
			return &q.tickets[i], nil
		}
	}

	return nil, fmt.Errorf("no waiting tickets")
}

// Skip marks a ticket as skipped
func (q *Queue) Skip(ticketId string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, t := range q.tickets {
		if t.ID == ticketId {
			if t.Status == "waiting" || t.Status == "called" {
				q.tickets[i].Status = "skipped"
				return nil
			}
			return fmt.Errorf("can only skip waiting or called tickets")
		}
	}
	return fmt.Errorf("ticket not found")
}

// Serve marks a ticket as "done"
func (q *Queue) Serve(ticketId string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, t := range q.tickets {
		if t.ID == ticketId {
			if t.Status == "called" {
				q.tickets[i].Status = "done"
				return nil
			}
			return fmt.Errorf("only called tickets can be served")
		}
	}
	return fmt.Errorf("ticket not found")
}

// GetDefault returns singleton queue instance
func GetDefault() *Queue {
	return defaultQueue
}
