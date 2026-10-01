package models

import "time"

// Role restricts values to exactly the two roles this system supports.
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

type User struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // never serialized into JSON responses
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}
