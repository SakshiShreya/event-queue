package store

type Status string

// room statuses
const (
	StatusOpen   Status = "open"
	StatusPaused Status = "paused"
	StatusClosed Status = "closed"
)

// Room represents one room's full queue
type Room struct {
	ID              string
	Name            string
	Status          Status
	AdmitRatePerMin int64
	NextNumber      int64
	AdmittedUpTo    int64
	CreatedAt       int64
	UpdatedAt       int64
}

// RoomState represents the hot fields that status path needs
type RoomState struct {
	Status          Status
	AdmitRatePerMin int64
	NextNumber      int64
	AdmittedUpTo    int64
}
