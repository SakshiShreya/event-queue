package queue

// Ticket represents a person in the queue
type Ticket struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
	Status   string `json:"status"`
	JoinedAt int64  `json:"joined_at"`
}

// Queue represents a single queue
type Queue struct {
	ID      string
	Tickets []Ticket
}

// TODO: Add queue operations (Join, Call, Skip, etc.)
