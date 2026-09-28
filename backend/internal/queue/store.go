package queue

type Store interface {
	Join(name string, partySize int) (*Ticket, error)
	Get(ticketId string) (*Ticket, error)
	GetAll() ([]Ticket, error)
	WaitingCount() (int, error)
	Call() (*Ticket, error)
	Skip(ticketId string) error
	Serve(ticketId string) error
}
