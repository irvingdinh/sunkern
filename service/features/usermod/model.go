package usermod

import "time"

// User represents a user account.
type User struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}
