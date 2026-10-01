package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"library-system/internal/models"
	"library-system/internal/repository"
	"library-system/internal/service"
)

// fakeUserRepository is a simple in-memory stand-in for UserRepository, so
// these tests run instantly and don't need a real database.
type fakeUserRepository struct {
	byEmail map[string]*models.User
	nextID  int
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{byEmail: make(map[string]*models.User)}
}

func (f *fakeUserRepository) Create(ctx context.Context, user *models.User) error {
	if _, exists := f.byEmail[user.Email]; exists {
		return repository.ErrEmailAlreadyExists
	}
	f.nextID++
	user.ID = f.nextID
	user.CreatedAt = time.Now()
	stored := *user
	f.byEmail[user.Email] = &stored
	return nil
}

func (f *fakeUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	user, ok := f.byEmail[email]
	if !ok {
		return nil, repository.ErrUserNotFound
	}
	return user, nil
}

func (f *fakeUserRepository) FindByID(ctx context.Context, id int) (*models.User, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, repository.ErrUserNotFound
}

func (f *fakeUserRepository) List(ctx context.Context) ([]models.User, error) {
	users := make([]models.User, 0, len(f.byEmail))
	for _, u := range f.byEmail {
		users = append(users, *u)
	}
	return users, nil
}

func TestRegisterAndLogin(t *testing.T) {
	repo := newFakeUserRepository()
	authService := service.NewAuthService(repo, "test-secret", time.Hour)

	user, err := authService.Register(context.Background(), service.RegisterInput{
		Name:     "Alice",
		Email:    "alice@example.com",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("expected register to succeed, got: %v", err)
	}
	if user.Role != models.RoleMember {
		t.Fatalf("expected new user to default to member role, got: %s", user.Role)
	}
	if user.PasswordHash == "secret123" {
		t.Fatal("password should have been hashed, not stored in plain text")
	}

	token, loggedInUser, err := authService.Login(context.Background(), "alice@example.com", "secret123")
	if err != nil {
		t.Fatalf("expected login to succeed, got: %v", err)
	}
	if token == "" {
		t.Fatal("expected a non-empty JWT")
	}
	if loggedInUser.Email != "alice@example.com" {
		t.Fatalf("expected logged-in user email to match, got: %s", loggedInUser.Email)
	}

	claims, err := authService.ParseToken(token)
	if err != nil {
		t.Fatalf("expected token to parse successfully, got: %v", err)
	}
	if claims.UserID != user.ID {
		t.Fatalf("expected claims.UserID %d, got %d", user.ID, claims.UserID)
	}
}

func TestLoginWithWrongPasswordFails(t *testing.T) {
	repo := newFakeUserRepository()
	authService := service.NewAuthService(repo, "test-secret", time.Hour)

	_, err := authService.Register(context.Background(), service.RegisterInput{
		Name:     "Bob",
		Email:    "bob@example.com",
		Password: "correct-password",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	_, _, err = authService.Login(context.Background(), "bob@example.com", "wrong-password")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestLoginWithUnknownEmailFails(t *testing.T) {
	repo := newFakeUserRepository()
	authService := service.NewAuthService(repo, "test-secret", time.Hour)

	_, _, err := authService.Login(context.Background(), "nobody@example.com", "whatever")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials for unknown email, got: %v", err)
	}
}

func TestRegisterDuplicateEmailFails(t *testing.T) {
	repo := newFakeUserRepository()
	authService := service.NewAuthService(repo, "test-secret", time.Hour)

	input := service.RegisterInput{Name: "Carol", Email: "carol@example.com", Password: "secret123"}
	if _, err := authService.Register(context.Background(), input); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	_, err := authService.Register(context.Background(), input)
	if !errors.Is(err, service.ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got: %v", err)
	}
}
