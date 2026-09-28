package queue

import "context"

type Store interface {
	Join(ctx context.Context, name string, partySize int) (*Ticket, error)
	Get(ctx context.Context, ticketId string) (*Ticket, error)
	GetAll(ctx context.Context) ([]Ticket, error)
	WaitingCount(ctx context.Context) (int, error)
	Call(ctx context.Context) (*Ticket, error)
	Skip(ctx context.Context, ticketId string) error
	Serve(ctx context.Context, ticketId string) error
}
