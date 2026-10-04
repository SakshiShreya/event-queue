package queue

import "context"

type Store interface {
	Join(ctx context.Context, name string, partySize int) (*Ticket, error)
	Get(ctx context.Context, ticketID string) (*Ticket, error)
	GetAll(ctx context.Context) ([]Ticket, error)
	WaitingCount(ctx context.Context) (int, error)
	Call(ctx context.Context) (*Ticket, error)
	Start(ctx context.Context, ticketID string) error
	Skip(ctx context.Context, ticketID string) error
	Done(ctx context.Context, ticketID string) error
}
