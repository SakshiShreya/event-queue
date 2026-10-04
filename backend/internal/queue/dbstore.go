package queue

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
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
		"SELECT COUNT(*) FROM Ticket WHERE queue_id = ? AND status = '"+StatusWaiting+"' AND id <= ?",
		s.queueID, id,
	).Scan(&position)

	return position, err
}

func (s *DBStore) Join(ctx context.Context, name string, partySize int) (*Ticket, error) {
	name = strings.TrimSpace(name)
	// Validate
	if name == "" {
		return nil, fmt.Errorf("%w: name cannot be empty", ErrValidation)
	}
	if utf8.RuneCountInString(name) > MaxNameLength {
		return nil, fmt.Errorf("%w: name can't be longer than %d characters", ErrValidation, MaxNameLength)
	}

	if partySize == 0 {
		partySize = 1
	}
	if partySize < 1 || partySize > MaxPartySize {
		return nil, fmt.Errorf("%w: party_size must be between 1 and %d", ErrValidation, MaxPartySize)
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

func (s *DBStore) Get(ctx context.Context, ticketID string) (*Ticket, error) {
	id, err := strconv.ParseInt(ticketID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, ticketID)
	}

	var displayName, status string
	var partySize int
	var joinedAt, calledAt, doneAt int64
	err = s.db.QueryRowContext(
		ctx,
		"SELECT id, display_name, party_size, status, joined_at, COALESCE(called_at, 0), COALESCE(done_at, 0) FROM Ticket WHERE queue_id = ? AND id = ?",
		s.queueID, id,
	).Scan(&id, &displayName, &partySize, &status, &joinedAt, &calledAt, &doneAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, ticketID)
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
		JoinedAt:  joinedAt,
		CalledAt:  calledAt,
		DoneAt:    doneAt,
		PartySize: partySize,
		Position:  position,
	}

	return ticket, nil
}

func (s *DBStore) GetAll(ctx context.Context) ([]Ticket, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, display_name, party_size, status, joined_at, COALESCE(called_at, 0), COALESCE(done_at, 0) FROM Ticket WHERE queue_id = ? ORDER BY id", s.queueID)
	if err != nil {
		return nil, fmt.Errorf("get all: %w", err)
	}
	defer rows.Close()

	tickets := make([]Ticket, 0)
	position := 0
	for rows.Next() {
		var id, joinedAt, calledAt, doneAt int64
		var displayName, status string
		var partySize int
		if err := rows.Scan(&id, &displayName, &partySize, &status, &joinedAt, &calledAt, &doneAt); err != nil {
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
			JoinedAt:  joinedAt,
			CalledAt:  calledAt,
			DoneAt:    doneAt,
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
		"SELECT COUNT(*) FROM Ticket WHERE queue_id = ? AND status = '"+StatusWaiting+"'",
		s.queueID,
	).Scan(&total)

	return total, err
}

func (s *DBStore) Call(ctx context.Context) (*Ticket, error) {
	var id int64
	err := s.db.QueryRowContext(
		ctx,
		"UPDATE Ticket SET status = '"+StatusCalled+"', called_at = ? WHERE id = (SELECT id FROM Ticket WHERE queue_id = ? AND status = '"+StatusWaiting+"' ORDER BY id LIMIT 1) RETURNING id",
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

// TRANSITION RULES
// status can only transition like this:
// Action	Allowed from	Moves to
// call		waiting			called
// skip		waiting, called	skipped
// start	called			serving
// done		serving			done
func (s *DBStore) transition(ctx context.Context, ticketID string, targetStatus string, startingStatuses ...string) error {
	id, err := strconv.ParseInt(ticketID, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrNotFound, ticketID)
	}

	startingStatusesPlaceholder := strings.TrimSuffix(strings.Repeat("?,", len(startingStatuses)), ",")
	args := []any{targetStatus}
	timestampPlaceholder := ""
	if targetStatus == StatusDone {
		timestampPlaceholder = ", done_at = ?"
		args = append(args, time.Now().Unix())
	}
	args = append(args, id, s.queueID)
	for _, status := range startingStatuses {
		args = append(args, status)
	}
	result, err := s.db.ExecContext(
		ctx,
		"UPDATE Ticket SET status = ?"+timestampPlaceholder+" WHERE id = ? AND queue_id = ? AND status IN ("+startingStatusesPlaceholder+")",
		args...,
	)
	if err != nil {
		return fmt.Errorf("transition ticket: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if count == 1 {
		return nil
	}

	ticket, err := s.Get(ctx, ticketID)
	if err != nil {
		return err
	}

	return fmt.Errorf("%w: can't move ticket from %s to %s", ErrConflict, ticket.Status, targetStatus)
}

func (s *DBStore) Start(ctx context.Context, ticketID string) error {
	return s.transition(ctx, ticketID, StatusServing, StatusCalled)
}

func (s *DBStore) Skip(ctx context.Context, ticketID string) error {
	return s.transition(ctx, ticketID, StatusSkipped, StatusWaiting, StatusCalled)
}

func (s *DBStore) Done(ctx context.Context, ticketID string) error {
	return s.transition(ctx, ticketID, StatusDone, StatusServing)
}

var _ Store = (*DBStore)(nil)
