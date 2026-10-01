package service

import (
	"context"
	"errors"

	"library-system/internal/models"
	"library-system/internal/repository"
)

var ErrUserNotFound = errors.New("user not found")

type UserService struct {
	userRepo repository.UserRepository
	loanRepo repository.LoanRepository
}

func NewUserService(userRepo repository.UserRepository, loanRepo repository.LoanRepository) *UserService {
	return &UserService{userRepo: userRepo, loanRepo: loanRepo}
}

func (s *UserService) List(ctx context.Context) ([]models.User, error) {
	return s.userRepo.List(ctx)
}

func (s *UserService) GetByID(ctx context.Context, id int) (*models.User, error) {
	user, err := s.userRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (s *UserService) GetLoanHistory(ctx context.Context, userID int) ([]models.Loan, error) {
	return s.loanRepo.ListByUser(ctx, userID)
}
