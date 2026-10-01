package repository_test

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"library-system/internal/repository"
)

// testPool connects to a real Postgres database for these integration
// tests. It reads DATABASE_URL from the environment — if it's not set,
// the test is skipped rather than failed, so `go test ./...` still works
// fine even if Postgres isn't running (e.g. in a quick CI check).
//
// Run these tests with something like:
//
//	DATABASE_URL="postgres://library_user:library_pass@localhost:5433/library_db?sslmode=disable" go test ./...
//
// (use whatever port your docker-compose.yml actually maps Postgres to)
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set — skipping integration test. See the comment at the top of this file for how to run it.")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// insertTestBook creates a throw-away book for this test and cleans it up
// (via CASCADE, this also removes any loans/reservations against it) when
// the test finishes, regardless of pass/fail.
func insertTestBook(t *testing.T, pool *pgxpool.Pool, totalCopies int) int {
	t.Helper()
	unique := strconv.FormatInt(time.Now().UnixNano(), 36)
	var id int
	err := pool.QueryRow(context.Background(), `
		INSERT INTO books (title, author, isbn, total_copies, available_copies)
		VALUES ($1, 'Test Author', $2, $3, $3)
		RETURNING id
	`, "Test Book "+unique, "T-"+unique, totalCopies).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test book: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM books WHERE id = $1`, id)
	})
	return id
}

func insertTestUser(t *testing.T, pool *pgxpool.Pool, label string) int {
	t.Helper()
	unique := time.Now().Format("150405.000000")
	email := "testuser-" + label + "-" + unique + "@example.com"
	var id int
	err := pool.QueryRow(context.Background(), `
		INSERT INTO users (name, email, password_hash, role)
		VALUES ('Test User', $1, 'not-a-real-hash', 'member')
		RETURNING id
	`, email).Scan(&id)
	if err != nil {
		t.Fatalf("failed to insert test user: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

func TestBorrowAndReturn(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewPostgresLoanRepository(pool)

	bookID := insertTestBook(t, pool, 1)
	userID := insertTestUser(t, pool, "borrow")

	loan, err := repo.Borrow(context.Background(), bookID, userID, time.Now().Add(14*24*time.Hour))
	if err != nil {
		t.Fatalf("expected borrow to succeed, got error: %v", err)
	}
	if loan.ID == 0 {
		t.Fatal("expected loan to have a nonzero id")
	}

	var available int
	pool.QueryRow(context.Background(), `SELECT available_copies FROM books WHERE id = $1`, bookID).Scan(&available)
	if available != 0 {
		t.Fatalf("expected 0 copies available after borrow, got %d", available)
	}

	returned, err := repo.Return(context.Background(), loan.ID)
	if err != nil {
		t.Fatalf("expected return to succeed, got error: %v", err)
	}
	if returned.ReturnedAt == nil {
		t.Fatal("expected returned_at to be set after returning")
	}

	pool.QueryRow(context.Background(), `SELECT available_copies FROM books WHERE id = $1`, bookID).Scan(&available)
	if available != 1 {
		t.Fatalf("expected 1 copy available after return, got %d", available)
	}
}

func TestReturningAlreadyReturnedLoanFails(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewPostgresLoanRepository(pool)

	bookID := insertTestBook(t, pool, 1)
	userID := insertTestUser(t, pool, "double-return")

	loan, err := repo.Borrow(context.Background(), bookID, userID, time.Now().Add(14*24*time.Hour))
	if err != nil {
		t.Fatalf("borrow failed: %v", err)
	}

	if _, err := repo.Return(context.Background(), loan.ID); err != nil {
		t.Fatalf("first return failed: %v", err)
	}

	if _, err := repo.Return(context.Background(), loan.ID); err != repository.ErrLoanAlreadyReturned {
		t.Fatalf("expected ErrLoanAlreadyReturned on second return, got: %v", err)
	}
}

func TestBorrowFailsWhenNoCopiesAvailable(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewPostgresLoanRepository(pool)

	bookID := insertTestBook(t, pool, 0) // zero copies from the start
	userID := insertTestUser(t, pool, "no-copies")

	_, err := repo.Borrow(context.Background(), bookID, userID, time.Now().Add(14*24*time.Hour))
	if err != repository.ErrNoCopiesAvailable {
		t.Fatalf("expected ErrNoCopiesAvailable, got: %v", err)
	}
}

// TestConcurrentBorrowRaceCondition is the important one: it proves the
// row-level locking (SELECT ... FOR UPDATE) inside Borrow() actually
// prevents two simultaneous requests from both succeeding when only one
// copy of a book exists. Without that locking, this test would be flaky
// and could show BOTH borrows succeeding (leaving available_copies at -1).
func TestConcurrentBorrowRaceCondition(t *testing.T) {
	pool := testPool(t)
	repo := repository.NewPostgresLoanRepository(pool)

	bookID := insertTestBook(t, pool, 1) // exactly ONE copy
	userA := insertTestUser(t, pool, "race-a")
	userB := insertTestUser(t, pool, "race-b")

	var wg sync.WaitGroup
	results := make(chan error, 2)

	attemptBorrow := func(userID int) {
		defer wg.Done()
		_, err := repo.Borrow(context.Background(), bookID, userID, time.Now().Add(14*24*time.Hour))
		results <- err
	}

	wg.Add(2)
	go attemptBorrow(userA)
	go attemptBorrow(userB)
	wg.Wait()
	close(results)

	successCount, failCount := 0, 0
	for err := range results {
		switch err {
		case nil:
			successCount++
		case repository.ErrNoCopiesAvailable:
			failCount++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if successCount != 1 || failCount != 1 {
		t.Fatalf("expected exactly 1 success and 1 ErrNoCopiesAvailable (race condition not handled safely), got %d successes and %d failures", successCount, failCount)
	}

	var available int
	pool.QueryRow(context.Background(), `SELECT available_copies FROM books WHERE id = $1`, bookID).Scan(&available)
	if available != 0 {
		t.Fatalf("expected exactly 0 copies available after the race, got %d (this would indicate a lost update)", available)
	}
}
