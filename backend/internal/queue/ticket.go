package queue

import "errors"

const MaxNameLength = 50
const MaxPartySize = 20

// Ticket statuses
const (
	StatusWaiting = "waiting"
	StatusCalled  = "called"
	StatusServing = "serving"
	StatusSkipped = "skipped"
	StatusDone    = "done"
)

// Sentinel Errors
var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("ticket not found")
	ErrConflict   = errors.New("conflict")
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
