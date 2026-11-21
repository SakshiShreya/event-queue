package store

import "errors"

// Sentinel Errors
var (
	ErrValidation   = errors.New("validation error")
	ErrNotFound     = errors.New("room not found")
	ErrRoomClosed   = errors.New("room is closed")
	ErrRoomNotOpen  = errors.New("room is not open")
	ErrUnauthorized = errors.New("invalid or missing token")
)
