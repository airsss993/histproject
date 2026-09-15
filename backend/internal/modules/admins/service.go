package admins

import (
	"context"
	"errors"
	"net/mail"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAdminNotFound    = errors.New("admin not found")
	ErrLoginTaken       = errors.New("admin login already exists")
	ErrSuperAdminExists = errors.New("super admin already exists")
	ErrSelfMutation     = errors.New("admin cannot mutate itself")
	ErrLastSuperAdmin   = errors.New("last super admin cannot be changed")
)

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type CreateInput struct {
	Login    string
	Password string
	Email    string
	Role     Role
}

type CreateRecord struct {
	Login        string
	PasswordHash string
	Email        string
	Role         Role
}

type ValidationError struct {
	Fields map[string]string
}

func (validationError *ValidationError) Error() string {
	return "admin validation failed"
}

type Store interface {
	Create(context.Context, CreateRecord) (Admin, error)
	CreateInitialSuperAdmin(context.Context, CreateRecord) (Admin, error)
	List(context.Context) ([]Admin, error)
	ChangeRole(context.Context, int64, Role) (Admin, error)
	Delete(context.Context, int64) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (service *Service) Create(ctx context.Context, input CreateInput) (Admin, error) {
	record, err := prepareCreateRecord(input)
	if err != nil {
		return Admin{}, err
	}
	return service.store.Create(ctx, record)
}

func (service *Service) Bootstrap(ctx context.Context, input CreateInput) (Admin, error) {
	input.Role = RoleSuperAdmin
	record, err := prepareCreateRecord(input)
	if err != nil {
		return Admin{}, err
	}
	return service.store.CreateInitialSuperAdmin(ctx, record)
}

func prepareCreateRecord(input CreateInput) (CreateRecord, error) {
	input.Login = strings.TrimSpace(input.Login)
	input.Email = strings.TrimSpace(input.Email)
	if fields := validateCreate(input); len(fields) > 0 {
		return CreateRecord{}, &ValidationError{Fields: fields}
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return CreateRecord{}, errors.New("hash admin password")
	}
	return CreateRecord{
		Login:        input.Login,
		PasswordHash: string(passwordHash),
		Email:        input.Email,
		Role:         input.Role,
	}, nil
}

func (service *Service) List(ctx context.Context) ([]Admin, error) {
	return service.store.List(ctx)
}

func (service *Service) ChangeRole(ctx context.Context, callerID, targetID int64, role Role) (Admin, error) {
	if callerID == targetID {
		return Admin{}, ErrSelfMutation
	}
	if !role.Valid() {
		return Admin{}, &ValidationError{Fields: map[string]string{"role": "Недопустимая роль"}}
	}
	return service.store.ChangeRole(ctx, targetID, role)
}

func (service *Service) Delete(ctx context.Context, callerID, targetID int64) error {
	if callerID == targetID {
		return ErrSelfMutation
	}
	return service.store.Delete(ctx, targetID)
}

func validateCreate(input CreateInput) map[string]string {
	fields := make(map[string]string)
	if len(input.Login) < 3 || len(input.Login) > 100 || !loginPattern.MatchString(input.Login) {
		fields["login"] = "От 3 до 100 латинских букв, цифр и символов ._-"
	}
	if len(input.Password) < 12 || len(input.Password) > 72 {
		fields["password"] = "От 12 до 72 байт: латинский символ занимает 1 байт, кириллический 2"
	}
	if input.Email != "" {
		address, err := mail.ParseAddress(input.Email)
		if err != nil || address.Address != input.Email {
			fields["email"] = "Некорректный email"
		}
	}
	if !input.Role.Valid() {
		fields["role"] = "Недопустимая роль"
	}
	return fields
}
