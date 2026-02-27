package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/msomdec/stitch-map-2/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles user registration, login, and JWT token operations.
type AuthService struct {
	users      domain.UserRepository
	jwtSecret  []byte
	bcryptCost int
}

// NewAuthService creates a new AuthService.
func NewAuthService(users domain.UserRepository, jwtSecret string, bcryptCost int) *AuthService {
	return &AuthService{
		users:      users,
		jwtSecret:  []byte(jwtSecret),
		bcryptCost: bcryptCost,
	}
}

// Register creates a new user account after validating inputs.
func (s *AuthService) Register(ctx context.Context, email, displayName, password, confirmPassword string) (*domain.User, error) {
	if email == "" || displayName == "" || password == "" {
		return nil, fmt.Errorf("%w: email, display name, and password are required", domain.ErrInvalidInput)
	}

	if len(email) > 254 {
		return nil, fmt.Errorf("%w: email must be 254 characters or fewer", domain.ErrInvalidInput)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, fmt.Errorf("%w: invalid email address format", domain.ErrInvalidInput)
	}
	if len(displayName) > 100 {
		return nil, fmt.Errorf("%w: display name must be 100 characters or fewer", domain.ErrInvalidInput)
	}

	if password != confirmPassword {
		return nil, fmt.Errorf("%w: passwords do not match", domain.ErrInvalidInput)
	}

	if len(password) < 8 {
		return nil, fmt.Errorf("%w: password must be at least 8 characters", domain.ErrInvalidInput)
	}
	if len(password) > 72 {
		return nil, fmt.Errorf("%w: password must be 72 characters or fewer", domain.ErrInvalidInput)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: string(hash),
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// dummyHash is a pre-computed bcrypt hash used to equalize timing when a user is
// not found, preventing user enumeration via response time analysis.
var dummyHash = func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("timing-equalization"), bcrypt.DefaultCost)
	return h
}()

// Login verifies credentials and returns a signed JWT token string.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// Perform a dummy bcrypt comparison to equalize timing with the
			// case where the user exists but the password is wrong.
			bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
			return "", domain.ErrUnauthorized
		}
		return "", fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", domain.ErrUnauthorized
	}

	token, err := s.generateJWT(user)
	if err != nil {
		return "", fmt.Errorf("generate jwt: %w", err)
	}

	return token, nil
}

// ValidateToken parses and validates a JWT token string.
// Returns the user ID from the sub claim.
func (s *AuthService) ValidateToken(tokenString string) (int64, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return 0, domain.ErrUnauthorized
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, domain.ErrUnauthorized
	}

	sub, err := claims.GetSubject()
	if err != nil {
		return 0, domain.ErrUnauthorized
	}

	userID, err := strconv.ParseInt(sub, 10, 64)
	if err != nil {
		return 0, domain.ErrUnauthorized
	}

	return userID, nil
}

// GetUserByID retrieves a user by their ID.
func (s *AuthService) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

func (s *AuthService) generateJWT(user *domain.User) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":          strconv.FormatInt(user.ID, 10),
		"email":        user.Email,
		"display_name": user.DisplayName,
		"iat":          now.Unix(),
		"exp":          now.Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
