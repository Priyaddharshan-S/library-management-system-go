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
	ErrBookNotFound      = errors.New("book not found")
	ErrISBNAlreadyExists = errors.New("isbn already exists")
)

type BookRepository interface {
	Create(ctx context.Context, book *models.Book) error
	FindByID(ctx context.Context, id int) (*models.Book, error)
	Update(ctx context.Context, book *models.Book) error
	Delete(ctx context.Context, id int) error
	List(ctx context.Context) ([]models.Book, error)
	Search(ctx context.Context, query string) ([]models.Book, error)
}

type PostgresBookRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresBookRepository(pool *pgxpool.Pool) *PostgresBookRepository {
	return &PostgresBookRepository{pool: pool}
}

func (r *PostgresBookRepository) Create(ctx context.Context, book *models.Book) error {
	query := `
		INSERT INTO books (title, author, isbn, total_copies, available_copies)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	err := r.pool.QueryRow(ctx, query,
		book.Title, book.Author, book.ISBN, book.TotalCopies, book.AvailableCopies,
	).Scan(&book.ID, &book.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return ErrISBNAlreadyExists
		}
		return err
	}
	return nil
}

func (r *PostgresBookRepository) FindByID(ctx context.Context, id int) (*models.Book, error) {
	query := `
		SELECT id, title, author, isbn, total_copies, available_copies, created_at
		FROM books
		WHERE id = $1
	`
	var book models.Book
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&book.ID, &book.Title, &book.Author, &book.ISBN,
		&book.TotalCopies, &book.AvailableCopies, &book.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBookNotFound
		}
		return nil, err
	}
	return &book, nil
}

func (r *PostgresBookRepository) Update(ctx context.Context, book *models.Book) error {
	query := `
		UPDATE books
		SET title = $1, author = $2, isbn = $3, total_copies = $4, available_copies = $5
		WHERE id = $6
	`
	tag, err := r.pool.Exec(ctx, query,
		book.Title, book.Author, book.ISBN, book.TotalCopies, book.AvailableCopies, book.ID,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrISBNAlreadyExists
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrBookNotFound
	}
	return nil
}

func (r *PostgresBookRepository) Delete(ctx context.Context, id int) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM books WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrBookNotFound
	}
	return nil
}

func (r *PostgresBookRepository) List(ctx context.Context) ([]models.Book, error) {
	query := `
		SELECT id, title, author, isbn, total_copies, available_copies, created_at
		FROM books
		ORDER BY title
	`
	return r.queryBooks(ctx, query)
}

// Search matches a partial, case-insensitive substring against title,
// author, OR isbn — e.g. searching "tolk" finds "J.R.R. Tolkien".
func (r *PostgresBookRepository) Search(ctx context.Context, term string) ([]models.Book, error) {
	query := `
		SELECT id, title, author, isbn, total_copies, available_copies, created_at
		FROM books
		WHERE title ILIKE '%' || $1 || '%'
		   OR author ILIKE '%' || $1 || '%'
		   OR isbn ILIKE '%' || $1 || '%'
		ORDER BY title
	`
	return r.queryBooks(ctx, query, term)
}

func (r *PostgresBookRepository) queryBooks(ctx context.Context, query string, args ...interface{}) ([]models.Book, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	books := []models.Book{}
	for rows.Next() {
		var b models.Book
		if err := rows.Scan(
			&b.ID, &b.Title, &b.Author, &b.ISBN, &b.TotalCopies, &b.AvailableCopies, &b.CreatedAt,
		); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}
