package admins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrAdminNotFound    = errors.New("admin not found")
	ErrLoginTaken       = errors.New("admin login already exists")
	ErrEmailTaken       = errors.New("admin email already exists")
	ErrUserNotFound     = errors.New("user account not found")
	ErrSuperAdminExists = errors.New("super admin already exists")
	ErrSelfMutation     = errors.New("admin cannot mutate itself")
	ErrLastSuperAdmin   = errors.New("last super admin cannot be changed")
)

var loginPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type CreateInput struct {
	Login    string
	Name     string
	Password string
	Email    string
	Role     Role
}

type CreateRecord struct {
	Login        string
	Name         string
	PasswordHash string
	Email        string
	Role         Role
}

// UpdateInput - правка роли. nil - поле не меняется.
type UpdateInput struct {
	Name  *string
	Email *string
}

type UpdateRecord struct {
	Name  *string
	Email *string
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
	Get(context.Context, int64) (Admin, error)
	Update(context.Context, int64, UpdateRecord) (Admin, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (service *Service) Create(ctx context.Context, input CreateInput) (Admin, error) {
	input.Name = strings.Join(strings.Fields(input.Name), " ")
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Role = Role(strings.TrimSpace(string(input.Role)))
	fields := validateCreate(input, true)
	delete(fields, "login")
	delete(fields, "password")
	if input.Email == "" {
		fields["email"] = "Введите почту"
	}
	if len(fields) > 0 {
		return Admin{}, &ValidationError{Fields: fields}
	}
	digest := sha256.Sum256([]byte(input.Email))
	record := CreateRecord{
		Login: "account-" + hex.EncodeToString(digest[:12]), Name: input.Name,
		PasswordHash: "disabled", Email: input.Email, Role: input.Role,
	}
	admin, err := service.store.Create(ctx, record)
	if err != nil {
		return Admin{}, err
	}
	return admin, nil
}

func (service *Service) Bootstrap(ctx context.Context, input CreateInput) (Admin, error) {
	input.Role = RoleSuperAdmin
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Email == "" {
		return Admin{}, &ValidationError{Fields: map[string]string{"email": "Введите почту"}}
	}
	record, err := prepareCreateRecord(input, false)
	if err != nil {
		return Admin{}, err
	}
	return service.store.CreateInitialSuperAdmin(ctx, record)
}

// Имя обязательно для новых модераторов; первый супер-админ из bootstrap-admin заводится без него.
func prepareCreateRecord(input CreateInput, requireName bool) (CreateRecord, error) {
	input.Login = strings.TrimSpace(input.Login)
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if fields := validateCreate(input, requireName); len(fields) > 0 {
		return CreateRecord{}, &ValidationError{Fields: fields}
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return CreateRecord{}, errors.New("hash admin password")
	}
	return CreateRecord{
		Login:        input.Login,
		Name:         input.Name,
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

// Update - правка чужой учётки супер-админом.
func (service *Service) Update(ctx context.Context, callerID, targetID int64, input UpdateInput) (Admin, error) {
	if callerID == targetID {
		return Admin{}, ErrSelfMutation
	}
	record, err := prepareUpdateRecord(input)
	if err != nil {
		return Admin{}, err
	}
	return service.store.Update(ctx, targetID, record)
}

func prepareUpdateRecord(input UpdateInput) (UpdateRecord, error) {
	fields := make(map[string]string)
	var record UpdateRecord
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if problem := nameProblem(name, true); problem != "" {
			fields["name"] = problem
		}
		record.Name = &name
	}
	if input.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*input.Email))
		if problem := emailProblem(email); problem != "" {
			fields["email"] = problem
		}
		if email == "" {
			fields["email"] = "Введите почту"
		}
		record.Email = &email
	}
	if len(fields) > 0 {
		return UpdateRecord{}, &ValidationError{Fields: fields}
	}
	return record, nil
}

func validateCreate(input CreateInput, requireName bool) map[string]string {
	fields := make(map[string]string)
	if len(input.Login) < 3 || len(input.Login) > 100 || !loginPattern.MatchString(input.Login) {
		fields["login"] = "От 3 до 100 латинских букв, цифр и символов ._-"
	}
	checks := map[string]string{
		"name": nameProblem(input.Name, requireName), "password": passwordProblem(input.Password), "email": emailProblem(input.Email),
	}
	for field, problem := range checks {
		if problem != "" {
			fields[field] = problem
		}
	}
	if !input.Role.Valid() {
		fields["role"] = "Недопустимая роль"
	}
	return fields
}

func nameProblem(name string, required bool) string {
	if required && name == "" {
		return "Введите имя"
	}
	if utf8.RuneCountInString(name) > 100 {
		return "Не более 100 символов"
	}
	return ""
}

func passwordProblem(password string) string {
	if len(password) < 12 || len(password) > 72 {
		return "От 12 до 72 байт: латинский символ занимает 1 байт, кириллический 2"
	}
	return ""
}

func emailProblem(email string) string {
	if email == "" {
		return ""
	}
	if address, err := mail.ParseAddress(email); err != nil || address.Address != email {
		return "Некорректный email"
	}
	return ""
}
