package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidSession     = errors.New("invalid session")
)

var dummyPasswordHash = []byte("$2a$10$7EqJtq98hPqEX7fNZaFWoO5rP7ARQkNua5kW7VZ6vKQZ8hS4TjK6e")

type CredentialReader interface {
	FindCredentialsByLogin(context.Context, string) (int64, string, string, string, error)
}

type SessionStore interface {
	Create(context.Context, string, Identity, time.Duration) error
	Get(context.Context, string) (Identity, error)
	Delete(context.Context, string) error
}

type tokenGenerator func() (string, error)

type Service struct {
	credentials CredentialReader
	sessions    SessionStore
	sessionTTL  time.Duration
	newToken    tokenGenerator
}

func NewService(credentials CredentialReader, sessions SessionStore, sessionTTL time.Duration) *Service {
	return newService(credentials, sessions, sessionTTL, newSessionToken)
}

func newService(credentials CredentialReader, sessions SessionStore, sessionTTL time.Duration, newToken tokenGenerator) *Service {
	return &Service{
		credentials: credentials,
		sessions:    sessions,
		sessionTTL:  sessionTTL,
		newToken:    newToken,
	}
}

func (service *Service) Login(ctx context.Context, login, password string) (string, Identity, error) {
	login = strings.TrimSpace(login)
	if login == "" || password == "" {
		return "", Identity{}, ErrInvalidCredentials
	}

	adminID, storedLogin, passwordHash, role, err := service.credentials.FindCredentialsByLogin(ctx, login)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
		return "", Identity{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", Identity{}, fmt.Errorf("find admin credentials: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return "", Identity{}, ErrInvalidCredentials
	}

	identity := Identity{AdminID: adminID, Login: storedLogin, Role: role}
	if !identity.Valid() {
		return "", Identity{}, ErrInvalidCredentials
	}
	token, err := service.newToken()
	if err != nil {
		return "", Identity{}, fmt.Errorf("generate session token: %w", err)
	}
	if err := service.sessions.Create(ctx, token, identity, service.sessionTTL); err != nil {
		return "", Identity{}, fmt.Errorf("create session: %w", err)
	}

	return token, identity, nil
}

func (service *Service) Authenticate(ctx context.Context, token string) (Identity, error) {
	if token == "" {
		return Identity{}, ErrInvalidSession
	}
	identity, err := service.sessions.Get(ctx, token)
	if errors.Is(err, ErrSessionNotFound) {
		return Identity{}, ErrInvalidSession
	}
	if err != nil {
		return Identity{}, fmt.Errorf("authenticate session: %w", err)
	}
	if !identity.Valid() {
		return Identity{}, ErrInvalidSession
	}
	// The session snapshot can outlive a deleted or re-roled admin, so the role always comes from the database.
	adminID, _, _, role, err := service.credentials.FindCredentialsByLogin(ctx, identity.Login)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && adminID != identity.AdminID) {
		_ = service.sessions.Delete(ctx, token)
		return Identity{}, ErrInvalidSession
	}
	if err != nil {
		return Identity{}, fmt.Errorf("authenticate admin: %w", err)
	}
	identity.Role = role
	if !identity.Valid() {
		return Identity{}, ErrInvalidSession
	}
	return identity, nil
}

func (service *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := service.sessions.Delete(ctx, token); err != nil {
		return fmt.Errorf("logout session: %w", err)
	}
	return nil
}

func (identity Identity) Valid() bool {
	return identity.AdminID > 0 && identity.Login != "" && (identity.Role == "admin" || identity.Role == "super_admin")
}

func newSessionToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
