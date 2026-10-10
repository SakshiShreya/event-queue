package store

import "errors"

// Sentinel Errors
var (
	ErrValidation = errors.New("validation error")
	ErrNotFound   = errors.New("room not found")
	ErrRoomClosed = errors.New("room is closed")
)
