package models

import "time"

type Loan struct {
	ID         int        `json:"id"`
	BookID     int        `json:"book_id"`
	UserID     int        `json:"user_id"`
	BorrowedAt time.Time  `json:"borrowed_at"`
	DueAt      time.Time  `json:"due_at"`
	ReturnedAt *time.Time `json:"returned_at,omitempty"` // nil means still on loan
	CreatedAt  time.Time  `json:"created_at"`
}

// IsOverdue is true only for loans that are still out AND past their due date.
func (l *Loan) IsOverdue() bool {
	return l.ReturnedAt == nil && time.Now().After(l.DueAt)
}
