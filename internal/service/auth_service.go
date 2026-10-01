package service

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"library-system/internal/models"
	"library-system/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already registered")
)

// Claims are the custom fields embedded in every JWT we issue.
type Claims struct {
	UserID int         `json:"user_id"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

type AuthService struct {
	userRepo  repository.UserRepository
	jwtSecret string
	jwtTTL    time.Duration
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string, jwtTTL time.Duration) *AuthService {
	return &AuthService{userRepo: userRepo, jwtSecret: jwtSecret, jwtTTL: jwtTTL}
}

type RegisterInput struct {
	Name     string
	Email    string
	Password string
	Role     models.Role // empty means "member" — see Register below
}

func (s *AuthService) Register(ctx context.Context, in RegisterInput) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	role := in.Role
	if role == "" {
		role = models.RoleMember
	}

	user := &models.User{
		Name:         in.Name,
		Email:        in.Email,
		PasswordHash: string(hash),
		Role:         role,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		if errors.Is(err, repository.ErrEmailAlreadyExists) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}

	return user, nil
}

// Login returns a signed JWT plus the user record on success.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, *models.User, error) {
	user, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return "", nil, ErrInvalidCredentials
		}
		return "", nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	token, err := s.generateToken(user)
	if err != nil {
		return "", nil, err
	}

	return token, user, nil
}

func (s *AuthService) generateToken(user *models.User) (string, error) {
	claims := Claims{
		UserID: user.ID,
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.jwtTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

// ParseToken verifies a token's signature and expiry, then returns its claims.
// Used by the auth middleware on every protected request.
func (s *AuthService) ParseToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, errors.New("invalid or expired token")
	}
	return claims, nil
}
