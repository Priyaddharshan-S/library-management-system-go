package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"library-system/internal/models"
)

var (
	ErrReservationNotFound = errors.New("reservation not found")
	ErrAlreadyReserved     = errors.New("you already have an active reservation for this book")
)

type ReservationRepository interface {
	Create(ctx context.Context, bookID, userID int) (*models.Reservation, error)
	Cancel(ctx context.Context, id, userID int, isAdmin bool) error
	ListByBook(ctx context.Context, bookID int) ([]models.Reservation, error)
	ListByUser(ctx context.Context, userID int) ([]models.Reservation, error)
	// NextWaiting returns the oldest still-waiting reservation for a book,
	// or ErrReservationNotFound if nobody is waiting.
	NextWaiting(ctx context.Context, bookID int) (*models.Reservation, error)
	MarkNotified(ctx context.Context, id int) error
}

type PostgresReservationRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresReservationRepository(pool *pgxpool.Pool) *PostgresReservationRepository {
	return &PostgresReservationRepository{pool: pool}
}

func (r *PostgresReservationRepository) Create(ctx context.Context, bookID, userID int) (*models.Reservation, error) {
	query := `
		INSERT INTO reservations (book_id, user_id, status)
		VALUES ($1, $2, 'waiting')
		RETURNING id, reserved_at
	`
	res := &models.Reservation{BookID: bookID, UserID: userID, Status: models.ReservationWaiting}
	err := r.pool.QueryRow(ctx, query, bookID, userID).Scan(&res.ID, &res.ReservedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation — this user already has a waiting reservation for this book
				return nil, ErrAlreadyReserved
			case "23503": // foreign_key_violation — book_id or user_id doesn't exist
				return nil, ErrBookNotFound
			}
		}
		return nil, err
	}
	return res, nil
}

// Cancel only cancels reservations still in "waiting" status. Members can
// only cancel their own; admins (isAdmin=true) can cancel anyone's.
func (r *PostgresReservationRepository) Cancel(ctx context.Context, id, userID int, isAdmin bool) error {
	query := `UPDATE reservations SET status = 'cancelled' WHERE id = $1 AND status = 'waiting'`
	args := []interface{}{id}
	if !isAdmin {
		query += ` AND user_id = $2`
		args = append(args, userID)
	}

	tag, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrReservationNotFound
	}
	return nil
}

func (r *PostgresReservationRepository) ListByBook(ctx context.Context, bookID int) ([]models.Reservation, error) {
	return r.queryReservations(ctx, `
		SELECT id, book_id, user_id, status, reserved_at, notified_at
		FROM reservations WHERE book_id = $1
		ORDER BY reserved_at
	`, bookID)
}

func (r *PostgresReservationRepository) ListByUser(ctx context.Context, userID int) ([]models.Reservation, error) {
	return r.queryReservations(ctx, `
		SELECT id, book_id, user_id, status, reserved_at, notified_at
		FROM reservations WHERE user_id = $1
		ORDER BY reserved_at DESC
	`, userID)
}

// NextWaiting uses FOR UPDATE SKIP LOCKED so that if two returns for the
// same book were somehow processed at the exact same moment, each one
// claims a different waiting reservation instead of both grabbing the same one.
func (r *PostgresReservationRepository) NextWaiting(ctx context.Context, bookID int) (*models.Reservation, error) {
	query := `
		SELECT id, book_id, user_id, status, reserved_at, notified_at
		FROM reservations
		WHERE book_id = $1 AND status = 'waiting'
		ORDER BY reserved_at
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`
	var res models.Reservation
	err := r.pool.QueryRow(ctx, query, bookID).Scan(
		&res.ID, &res.BookID, &res.UserID, &res.Status, &res.ReservedAt, &res.NotifiedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReservationNotFound
		}
		return nil, err
	}
	return &res, nil
}

func (r *PostgresReservationRepository) MarkNotified(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE reservations SET status = 'notified', notified_at = now() WHERE id = $1`, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrReservationNotFound
	}
	return nil
}

func (r *PostgresReservationRepository) queryReservations(ctx context.Context, query string, args ...interface{}) ([]models.Reservation, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reservations := []models.Reservation{}
	for rows.Next() {
		var res models.Reservation
		if err := rows.Scan(
			&res.ID, &res.BookID, &res.UserID, &res.Status, &res.ReservedAt, &res.NotifiedAt,
		); err != nil {
			return nil, err
		}
		reservations = append(reservations, res)
	}
	return reservations, rows.Err()
}
