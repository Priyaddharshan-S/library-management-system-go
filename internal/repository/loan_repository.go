package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"library-system/internal/models"
)

var (
	ErrLoanNotFound        = errors.New("loan not found")
	ErrLoanAlreadyReturned = errors.New("loan already returned")
	ErrNoCopiesAvailable   = errors.New("no copies available")
)

type LoanRepository interface {
	Borrow(ctx context.Context, bookID, userID int, dueAt time.Time) (*models.Loan, error)
	Return(ctx context.Context, loanID int) (*models.Loan, error)
	FindByID(ctx context.Context, id int) (*models.Loan, error)
	ListActive(ctx context.Context) ([]models.Loan, error)
	ListOverdue(ctx context.Context) ([]models.Loan, error)
	ListByUser(ctx context.Context, userID int) ([]models.Loan, error)
}

type PostgresLoanRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresLoanRepository(pool *pgxpool.Pool) *PostgresLoanRepository {
	return &PostgresLoanRepository{pool: pool}
}

// Borrow is the concurrency-safe core of the whole system. It runs inside a
// single database transaction and uses "SELECT ... FOR UPDATE" to lock the
// book's row for the duration of the transaction.
//
// Why this matters: imagine a book has exactly 1 copy left, and two users
// click "borrow" at almost the same instant. Without the lock, both
// requests could read available_copies = 1 at the same time, both think
// there's a copy free, and both succeed — leaving available_copies at -1.
// With FOR UPDATE, the second transaction is forced to WAIT until the
// first one commits or rolls back. By the time it proceeds, it re-reads
// available_copies (now 0) and correctly fails with ErrNoCopiesAvailable.
func (r *PostgresLoanRepository) Borrow(ctx context.Context, bookID, userID int, dueAt time.Time) (*models.Loan, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) // no-op once Commit succeeds

	var available int
	err = tx.QueryRow(ctx, `SELECT available_copies FROM books WHERE id = $1 FOR UPDATE`, bookID).
		Scan(&available)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBookNotFound
		}
		return nil, err
	}

	if available < 1 {
		return nil, ErrNoCopiesAvailable
	}

	if _, err := tx.Exec(ctx,
		`UPDATE books SET available_copies = available_copies - 1 WHERE id = $1`, bookID,
	); err != nil {
		return nil, err
	}

	loan := &models.Loan{BookID: bookID, UserID: userID, DueAt: dueAt}
	err = tx.QueryRow(ctx, `
		INSERT INTO loans (book_id, user_id, due_at)
		VALUES ($1, $2, $3)
		RETURNING id, borrowed_at, created_at
	`, bookID, userID, dueAt).Scan(&loan.ID, &loan.BorrowedAt, &loan.CreatedAt)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return loan, nil
}

// Return also runs in a transaction: it locks the specific loan row so two
// simultaneous "return" calls for the same loan can't both succeed and
// double-increment available_copies.
func (r *PostgresLoanRepository) Return(ctx context.Context, loanID int) (*models.Loan, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var loan models.Loan
	err = tx.QueryRow(ctx, `
		SELECT id, book_id, user_id, borrowed_at, due_at, returned_at, created_at
		FROM loans WHERE id = $1 FOR UPDATE
	`, loanID).Scan(
		&loan.ID, &loan.BookID, &loan.UserID, &loan.BorrowedAt, &loan.DueAt, &loan.ReturnedAt, &loan.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLoanNotFound
		}
		return nil, err
	}

	if loan.ReturnedAt != nil {
		return nil, ErrLoanAlreadyReturned
	}

	now := time.Now()
	if _, err := tx.Exec(ctx, `UPDATE loans SET returned_at = $1 WHERE id = $2`, now, loanID); err != nil {
		return nil, err
	}
	loan.ReturnedAt = &now

	if _, err := tx.Exec(ctx,
		`UPDATE books SET available_copies = available_copies + 1 WHERE id = $1`, loan.BookID,
	); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &loan, nil
}

func (r *PostgresLoanRepository) FindByID(ctx context.Context, id int) (*models.Loan, error) {
	query := `
		SELECT id, book_id, user_id, borrowed_at, due_at, returned_at, created_at
		FROM loans WHERE id = $1
	`
	var loan models.Loan
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&loan.ID, &loan.BookID, &loan.UserID, &loan.BorrowedAt, &loan.DueAt, &loan.ReturnedAt, &loan.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLoanNotFound
		}
		return nil, err
	}
	return &loan, nil
}

func (r *PostgresLoanRepository) ListActive(ctx context.Context) ([]models.Loan, error) {
	return r.queryLoans(ctx, `
		SELECT id, book_id, user_id, borrowed_at, due_at, returned_at, created_at
		FROM loans WHERE returned_at IS NULL
		ORDER BY due_at
	`)
}

func (r *PostgresLoanRepository) ListOverdue(ctx context.Context) ([]models.Loan, error) {
	return r.queryLoans(ctx, `
		SELECT id, book_id, user_id, borrowed_at, due_at, returned_at, created_at
		FROM loans WHERE returned_at IS NULL AND due_at < now()
		ORDER BY due_at
	`)
}

func (r *PostgresLoanRepository) ListByUser(ctx context.Context, userID int) ([]models.Loan, error) {
	return r.queryLoans(ctx, `
		SELECT id, book_id, user_id, borrowed_at, due_at, returned_at, created_at
		FROM loans WHERE user_id = $1
		ORDER BY borrowed_at DESC
	`, userID)
}

func (r *PostgresLoanRepository) queryLoans(ctx context.Context, query string, args ...interface{}) ([]models.Loan, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	loans := []models.Loan{}
	for rows.Next() {
		var l models.Loan
		if err := rows.Scan(
			&l.ID, &l.BookID, &l.UserID, &l.BorrowedAt, &l.DueAt, &l.ReturnedAt, &l.CreatedAt,
		); err != nil {
			return nil, err
		}
		loans = append(loans, l)
	}
	return loans, rows.Err()
}
