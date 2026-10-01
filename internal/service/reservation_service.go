package service

import (
	"context"
	"errors"

	"library-system/internal/models"
	"library-system/internal/repository"
)

var (
	ErrAlreadyReserved     = errors.New("you already have an active reservation for this book")
	ErrReservationNotFound = errors.New("reservation not found")
)

type ReservationService struct {
	reservationRepo repository.ReservationRepository
	bookRepo        repository.BookRepository
}

func NewReservationService(reservationRepo repository.ReservationRepository, bookRepo repository.BookRepository) *ReservationService {
	return &ReservationService{reservationRepo: reservationRepo, bookRepo: bookRepo}
}

func (s *ReservationService) Reserve(ctx context.Context, bookID, userID int) (*models.Reservation, error) {
	if _, err := s.bookRepo.FindByID(ctx, bookID); err != nil {
		if errors.Is(err, repository.ErrBookNotFound) {
			return nil, ErrBookNotFound
		}
		return nil, err
	}

	res, err := s.reservationRepo.Create(ctx, bookID, userID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrAlreadyReserved):
			return nil, ErrAlreadyReserved
		case errors.Is(err, repository.ErrBookNotFound):
			return nil, ErrBookNotFound
		default:
			return nil, err
		}
	}
	return res, nil
}

func (s *ReservationService) Cancel(ctx context.Context, id, userID int, isAdmin bool) error {
	if err := s.reservationRepo.Cancel(ctx, id, userID, isAdmin); err != nil {
		if errors.Is(err, repository.ErrReservationNotFound) {
			return ErrReservationNotFound
		}
		return err
	}
	return nil
}

func (s *ReservationService) ListByUser(ctx context.Context, userID int) ([]models.Reservation, error) {
	return s.reservationRepo.ListByUser(ctx, userID)
}

func (s *ReservationService) ListByBook(ctx context.Context, bookID int) ([]models.Reservation, error) {
	return s.reservationRepo.ListByBook(ctx, bookID)
}
