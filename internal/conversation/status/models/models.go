package models

import "time"

var DefaultStatuses = []string{
	"Open",
	"Snoozed",
	"Resolved",
	"Closed",
}

const (
	CategoryOpen     = "open"
	CategoryWaiting  = "waiting"
	CategoryResolved = "resolved"
)

var ValidCategories = []string{
	CategoryOpen,
	CategoryWaiting,
	CategoryResolved,
}

// Allowed colour keys for the status pill. Frontend has the matching palette
// (15 Tailwind-derived presets); backend just persists the key string. Treated
// as freeform on the backend so future palette changes don't require a schema
// migration, but Create/Update default empty values to "gray" so older clients
// that don't send a color still get a valid render.
type Status struct {
	ID        int       `db:"id" json:"id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	Name      string    `db:"name" json:"name"`
	Category  string    `db:"category" json:"category"`
	Color     string    `db:"color" json:"color"`
}
