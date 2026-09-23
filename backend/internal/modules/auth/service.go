package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

var (
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrNotAdmin        = errors.New("user is not an admin")
)

type Identity struct {
	AdminID int64
	Login   string
	Name    string
	Role    string
}

type AdminReader interface {
	FindIdentityByEmail(context.Context, string) (int64, string, string, string, error)
}

type Service struct {
	admins AdminReader
}

func NewService(admins AdminReader) *Service {
	return &Service{admins: admins}
}

func (service *Service) Resolve(ctx context.Context, email string) (Identity, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return Identity{}, ErrUnauthenticated
	}
	adminID, login, name, role, err := service.admins.FindIdentityByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, ErrNotAdmin
	}
	if err != nil {
		return Identity{}, fmt.Errorf("find admin by user email: %w", err)
	}
	identity := Identity{AdminID: adminID, Login: login, Name: name, Role: role}
	if !identity.Valid() {
		return Identity{}, ErrNotAdmin
	}
	return identity, nil
}

func (identity Identity) Valid() bool {
	return identity.AdminID > 0 && identity.Login != "" && (identity.Role == "admin" || identity.Role == "super_admin")
}
