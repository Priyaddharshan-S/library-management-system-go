package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"library-system/internal/models"
	"library-system/internal/notification"
	"library-system/internal/repository"
)

const DefaultLoanDays = 14

var (
	ErrLoanNotFound        = errors.New("loan not found")
	ErrLoanAlreadyReturned = errors.New("loan already returned")
	ErrNoCopiesAvailable   = errors.New("no copies available")
)

type LoanService struct {
	loanRepo        repository.LoanRepository
	bookRepo        repository.BookRepository
	userRepo        repository.UserRepository
	reservationRepo repository.ReservationRepository
	notifier        notification.Sender
	loanDays        int
}

func NewLoanService(
	loanRepo repository.LoanRepository,
	bookRepo repository.BookRepository,
	userRepo repository.UserRepository,
	reservationRepo repository.ReservationRepository,
	notifier notification.Sender,
	loanDays int,
) *LoanService {
	if loanDays <= 0 {
		loanDays = DefaultLoanDays
	}
	return &LoanService{
		loanRepo:        loanRepo,
		bookRepo:        bookRepo,
		userRepo:        userRepo,
		reservationRepo: reservationRepo,
		notifier:        notifier,
		loanDays:        loanDays,
	}
}

func (s *LoanService) Borrow(ctx context.Context, bookID, userID int) (*models.Loan, error) {
	dueAt := time.Now().AddDate(0, 0, s.loanDays)

	loan, err := s.loanRepo.Borrow(ctx, bookID, userID, dueAt)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrBookNotFound):
			return nil, ErrBookNotFound
		case errors.Is(err, repository.ErrNoCopiesAvailable):
			return nil, ErrNoCopiesAvailable
		default:
			return nil, err
		}
	}
	return loan, nil
}

func (s *LoanService) Return(ctx context.Context, loanID int) (*models.Loan, error) {
	loan, err := s.loanRepo.Return(ctx, loanID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrLoanNotFound):
			return nil, ErrLoanNotFound
		case errors.Is(err, repository.ErrLoanAlreadyReturned):
			return nil, ErrLoanAlreadyReturned
		default:
			return nil, err
		}
	}

	// Best-effort: the return itself already succeeded, so a notification
	// failure (no SMTP configured, network hiccup, etc.) should never make
	// the whole Return() call fail — we just log it and move on.
	s.notifyNextInLine(ctx, loan.BookID)

	return loan, nil
}

func (s *LoanService) notifyNextInLine(ctx context.Context, bookID int) {
	reservation, err := s.reservationRepo.NextWaiting(ctx, bookID)
	if err != nil {
		return // nobody waiting — nothing to do
	}

	book, err := s.bookRepo.FindByID(ctx, bookID)
	if err != nil {
		log.Printf("notifyNextInLine: failed to load book %d: %v", bookID, err)
		return
	}

	user, err := s.userRepo.FindByID(ctx, reservation.UserID)
	if err != nil {
		log.Printf("notifyNextInLine: failed to load user %d: %v", reservation.UserID, err)
		return
	}

	subject := fmt.Sprintf("%s is now available", book.Title)
	body := fmt.Sprintf(
		"Hi %s,\n\nGood news — \"%s\" by %s is now available for pickup. Please visit the library soon to borrow it.\n\n- Library System",
		user.Name, book.Title, book.Author,
	)

	if err := s.notifier.Send(user.Email, subject, body); err != nil {
		log.Printf("notifyNextInLine: failed to send email to %s: %v", user.Email, err)
		return
	}

	if err := s.reservationRepo.MarkNotified(ctx, reservation.ID); err != nil {
		log.Printf("notifyNextInLine: failed to mark reservation %d notified: %v", reservation.ID, err)
	}
}

func (s *LoanService) ListActive(ctx context.Context) ([]models.Loan, error) {
	return s.loanRepo.ListActive(ctx)
}

func (s *LoanService) ListOverdue(ctx context.Context) ([]models.Loan, error) {
	return s.loanRepo.ListOverdue(ctx)
}

func (s *LoanService) ListByUser(ctx context.Context, userID int) ([]models.Loan, error) {
	return s.loanRepo.ListByUser(ctx, userID)
}
