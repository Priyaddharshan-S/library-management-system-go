package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"library-system/internal/models"
)

type ReportRepository interface {
	GetSummary(ctx context.Context) (*models.ReportSummary, error)
}

type PostgresReportRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresReportRepository(pool *pgxpool.Pool) *PostgresReportRepository {
	return &PostgresReportRepository{pool: pool}
}

func (r *PostgresReportRepository) GetSummary(ctx context.Context) (*models.ReportSummary, error) {
	summary := &models.ReportSummary{}

	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(available_copies), 0) FROM books`,
	).Scan(&summary.TotalBooks, &summary.AvailableBooks)
	if err != nil {
		return nil, err
	}

	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM loans WHERE returned_at IS NULL`,
	).Scan(&summary.ActiveLoans)
	if err != nil {
		return nil, err
	}

	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM loans WHERE returned_at IS NULL AND due_at < now()`,
	).Scan(&summary.OverdueLoans)
	if err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT b.id, b.title, b.author, COUNT(l.id) AS borrow_count
		FROM loans l
		JOIN books b ON b.id = l.book_id
		GROUP BY b.id, b.title, b.author
		ORDER BY borrow_count DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary.TopBorrowedBooks = []models.TopBook{}
	for rows.Next() {
		var tb models.TopBook
		if err := rows.Scan(&tb.BookID, &tb.Title, &tb.Author, &tb.BorrowCount); err != nil {
			return nil, err
		}
		summary.TopBorrowedBooks = append(summary.TopBorrowedBooks, tb)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return summary, nil
}
