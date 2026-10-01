package service

import (
	"context"
	"errors"

	"library-system/internal/models"
	"library-system/internal/repository"
)

var (
	ErrBookNotFound      = errors.New("book not found")
	ErrISBNTaken         = errors.New("isbn already exists")
	ErrInvalidCopyCount  = errors.New("total_copies must be at least 1")
	ErrCannotReduceBelow = errors.New("cannot reduce total_copies below the number currently on loan")
)

type BookService struct {
	bookRepo repository.BookRepository
}

func NewBookService(bookRepo repository.BookRepository) *BookService {
	return &BookService{bookRepo: bookRepo}
}

type CreateBookInput struct {
	Title       string
	Author      string
	ISBN        string
	TotalCopies int
}

func (s *BookService) Create(ctx context.Context, in CreateBookInput) (*models.Book, error) {
	if in.TotalCopies < 1 {
		return nil, ErrInvalidCopyCount
	}

	book := &models.Book{
		Title:           in.Title,
		Author:          in.Author,
		ISBN:            in.ISBN,
		TotalCopies:     in.TotalCopies,
		AvailableCopies: in.TotalCopies, // a brand-new book starts fully available
	}

	if err := s.bookRepo.Create(ctx, book); err != nil {
		if errors.Is(err, repository.ErrISBNAlreadyExists) {
			return nil, ErrISBNTaken
		}
		return nil, err
	}
	return book, nil
}

func (s *BookService) GetByID(ctx context.Context, id int) (*models.Book, error) {
	book, err := s.bookRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrBookNotFound) {
			return nil, ErrBookNotFound
		}
		return nil, err
	}
	return book, nil
}

func (s *BookService) List(ctx context.Context) ([]models.Book, error) {
	return s.bookRepo.List(ctx)
}

func (s *BookService) Search(ctx context.Context, term string) ([]models.Book, error) {
	return s.bookRepo.Search(ctx, term)
}

type UpdateBookInput struct {
	Title       string
	Author      string
	ISBN        string
	TotalCopies int
}

// Update changes book details. If TotalCopies changes, AvailableCopies is
// shifted by the same delta, so copies currently on loan are preserved
// correctly (e.g. 5 total/2 available, on loan = 3; raise total to 7 ->
// available becomes 4, still 3 on loan).
func (s *BookService) Update(ctx context.Context, id int, in UpdateBookInput) (*models.Book, error) {
	if in.TotalCopies < 1 {
		return nil, ErrInvalidCopyCount
	}

	existing, err := s.bookRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrBookNotFound) {
			return nil, ErrBookNotFound
		}
		return nil, err
	}

	onLoan := existing.TotalCopies - existing.AvailableCopies
	if in.TotalCopies < onLoan {
		return nil, ErrCannotReduceBelow
	}

	existing.Title = in.Title
	existing.Author = in.Author
	existing.ISBN = in.ISBN
	existing.TotalCopies = in.TotalCopies
	existing.AvailableCopies = in.TotalCopies - onLoan

	if err := s.bookRepo.Update(ctx, existing); err != nil {
		if errors.Is(err, repository.ErrISBNAlreadyExists) {
			return nil, ErrISBNTaken
		}
		if errors.Is(err, repository.ErrBookNotFound) {
			return nil, ErrBookNotFound
		}
		return nil, err
	}
	return existing, nil
}

func (s *BookService) Delete(ctx context.Context, id int) error {
	if err := s.bookRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrBookNotFound) {
			return ErrBookNotFound
		}
		return err
	}
	return nil
}
