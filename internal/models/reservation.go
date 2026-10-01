package models

import "time"

type ReservationStatus string

const (
	ReservationWaiting   ReservationStatus = "waiting"
	ReservationNotified  ReservationStatus = "notified"
	ReservationFulfilled ReservationStatus = "fulfilled"
	ReservationCancelled ReservationStatus = "cancelled"
)

type Reservation struct {
	ID         int               `json:"id"`
	BookID     int               `json:"book_id"`
	UserID     int               `json:"user_id"`
	Status     ReservationStatus `json:"status"`
	ReservedAt time.Time         `json:"reserved_at"`
	NotifiedAt *time.Time        `json:"notified_at,omitempty"`
}
