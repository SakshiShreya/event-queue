package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type DBStore struct {
	db      *sql.DB
	queueID string
}

func NewDBStore(db *sql.DB, queueID string) *DBStore {
	return &DBStore{db, queueID}
}

func (s *DBStore) position(ctx context.Context, id int64) (int, error) {
	var position int
	err := s.db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM Ticket WHERE queue_id = ? AND status = 'waiting' AND id <= ?",
		s.queueID, id,
	).Scan(&position)

	return position, err
}

func (s *DBStore) Join(ctx context.Context, name string, partySize int) (*Ticket, error) {
	// Validate
	if name == "" {
		return nil, fmt.Errorf("%w: name cannot be empty", ErrValidation)
	}
	if partySize <= 0 {
		return nil, fmt.Errorf("%w: party_size must be > 0", ErrValidation)
	}

	now := time.Now().Unix()

	result, err := s.db.ExecContext(
		ctx,
		"INSERT INTO Ticket (queue_id, display_name, party_size, joined_at) VALUES (?, ?, ?, ?)",
		s.queueID, name, partySize, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert ticket: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get id: %w", err)
	}

	position, err := s.position(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("count position: %w", err)
	}

	// create ticket
	ticket := &Ticket{
		ID:        strconv.FormatInt(id, 10),
		Name:      name,
		Status:    StatusWaiting,
		JoinedAt:  now,
		PartySize: partySize,
		Position:  position,
	}

	return ticket, nil
}

func (s *DBStore) Get(ctx context.Context, ticketId string) (*Ticket, error) {
	id, err := strconv.ParseInt(ticketId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, ticketId)
	}

	var displayName, status string
	var partySize, joinedAt int
	err = s.db.QueryRowContext(
		ctx,
		"SELECT id, display_name, party_size, status, joined_at FROM Ticket WHERE queue_id = ? AND id = ?",
		s.queueID, id,
	).Scan(&id, &displayName, &partySize, &status, &joinedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, ticketId)
		}
		return nil, fmt.Errorf("get ticket: %w", err)
	}

	position := 0
	if status == StatusWaiting {
		position, err = s.position(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("count position: %w", err)
		}
	}

	// create ticket
	ticket := &Ticket{
		ID:        strconv.FormatInt(id, 10),
		Name:      displayName,
		Status:    status,
		JoinedAt:  int64(joinedAt),
		PartySize: partySize,
		Position:  position,
	}

	return ticket, nil
}

func (s *DBStore) GetAll(ctx context.Context) ([]Ticket, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, display_name, party_size, status, joined_at FROM Ticket WHERE queue_id = ? ORDER BY id", s.queueID)
	if err != nil {
		return nil, fmt.Errorf("get all: %w", err)
	}
	defer rows.Close()

	tickets := make([]Ticket, 0)
	position := 0
	for rows.Next() {
		var id, joinedAt int64
		var displayName, status string
		var partySize int
		if err := rows.Scan(&id, &displayName, &partySize, &status, &joinedAt); err != nil {
			return nil, fmt.Errorf("scan ticket %w", err)
		}

		if status == StatusWaiting {
			position++
		}

		ticketPosition := position
		if status != StatusWaiting {
			ticketPosition = 0
		}

		ticket := Ticket{
			ID:        strconv.FormatInt(id, 10),
			Name:      displayName,
			Status:    status,
			JoinedAt:  int64(joinedAt),
			PartySize: partySize,
			Position:  ticketPosition,
		}
		tickets = append(tickets, ticket)
	}
	return tickets, rows.Err()
}

func (s *DBStore) WaitingCount(ctx context.Context) (int, error) {
	var total int
	err := s.db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM Ticket WHERE queue_id = ? AND status = 'waiting'",
		s.queueID,
	).Scan(&total)

	return total, err
}

func (s *DBStore) Call(ctx context.Context) (*Ticket, error) {
	var id int64
	err := s.db.QueryRowContext(
		ctx,
		"UPDATE Ticket SET status = 'called', called_at = ? WHERE id = (SELECT id FROM Ticket WHERE queue_id = ? AND status = 'waiting' ORDER BY id LIMIT 1) RETURNING id",
		time.Now().Unix(), s.queueID,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: no waiting tickets", ErrConflict)
		}
		return nil, fmt.Errorf("call ticket: %w", err)
	}

	return s.Get(ctx, strconv.FormatInt(id, 10))
}

func (s *DBStore) Skip(ctx context.Context, ticketId string) error {
	result, err := s.db.ExecContext(
		ctx,
		"UPDATE Ticket SET status = 'skipped' WHERE id = (SELECT id FROM Ticket WHERE queue_id = ? AND status IN ('waiting','called') AND id = ?)",
		s.queueID, ticketId,
	)
	if err != nil {
		return fmt.Errorf("skip ticket: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if count == 1 {
		return nil
	}

	ticket, err := s.Get(ctx, ticketId)
	if err != nil {
		return err
	}

	return fmt.Errorf("%w: can only skip waiting or called tickets, got %q", ErrConflict, ticket.Status)
}

func (s *DBStore) Serve(ctx context.Context, ticketId string) error {
	result, err := s.db.ExecContext(
		ctx,
		"UPDATE Ticket SET status = 'done', done_at = ? WHERE id = (SELECT id FROM Ticket WHERE queue_id = ? AND status = 'called' AND id = ?)",
		time.Now().Unix(), s.queueID, ticketId,
	)
	if err != nil {
		return fmt.Errorf("serve ticket: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if count == 1 {
		return nil
	}

	ticket, err := s.Get(ctx, ticketId)
	if err != nil {
		return err
	}

	return fmt.Errorf("%w: only called tickets can be served, got %q", ErrConflict, ticket.Status)
}

var _ Store = (*DBStore)(nil)
