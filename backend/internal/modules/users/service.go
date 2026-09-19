package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	codeTTL         = 10 * time.Minute
	codeAttempts    = 5
	codeCooldown    = time.Minute
	ipCodeLimit     = 10
	ipCodeWindow    = 10 * time.Minute
	RememberTTL     = 30 * 24 * time.Hour
	nameMaxRunes    = 50
	emailMaxRunes   = 254
	sessionTokenLen = 32
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrEmailTaken   = errors.New("email taken")
	ErrInvalidCode  = errors.New("invalid code")
	ErrCodeExpired  = errors.New("code expired")
	ErrRateLimited  = errors.New("rate limited")
	ErrNoSession    = errors.New("no session")
)

type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	CreatedAt time.Time `json:"createdAt"`
}

// Pending - то, что ждёт подтверждения кодом. Имя заполнено только у регистрации.
type Pending struct {
	CodeHash  string
	FirstName string
	LastName  string
}

type ValidationError map[string]string

func (ValidationError) Error() string { return "validation failed" }

type Repository interface {
	FindByEmail(context.Context, string) (User, error)
	FindByID(context.Context, int64) (User, error)
	Create(context.Context, User) (User, error)
}

type Store interface {
	SaveCode(context.Context, string, Pending, time.Duration) error
	// CheckCode сверяет хеш и удаляет код при успехе. ErrInvalidCode - не совпал, ErrCodeExpired - нет кода или попытки кончились.
	CheckCode(context.Context, string, string, int) (Pending, error)
	Allow(context.Context, string, int, time.Duration) (bool, error)
	CreateSession(context.Context, string, int64, time.Duration) error
	SessionUser(context.Context, string) (int64, error)
	DeleteSession(context.Context, string) error
}

type Mailer interface {
	SendCode(ctx context.Context, email, code string) error
}

type Service struct {
	repository Repository
	store      Store
	mailer     Mailer
	sessionTTL time.Duration
	newCode    func() (string, error)
}

func NewService(repository Repository, store Store, mailer Mailer, sessionTTL time.Duration) *Service {
	return &Service{repository: repository, store: store, mailer: mailer, sessionTTL: sessionTTL, newCode: newCode}
}

type RegisterInput struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
}

// Register шлёт код на почту. Учётка появится только после Verify: неподтверждённых записей в базе нет.
func (service *Service) Register(ctx context.Context, input RegisterInput, clientIP string) error {
	fields := ValidationError{}
	firstName := validateName(fields, "firstName", input.FirstName)
	lastName := validateName(fields, "lastName", input.LastName)
	email := validateEmail(fields, input.Email)
	if len(fields) > 0 {
		return fields
	}
	if _, err := service.repository.FindByEmail(ctx, email); err == nil {
		return ErrEmailTaken
	} else if !errors.Is(err, ErrUserNotFound) {
		return err
	}
	return service.sendCode(ctx, email, clientIP, Pending{FirstName: firstName, LastName: lastName})
}

func (service *Service) RequestLogin(ctx context.Context, rawEmail, clientIP string) error {
	fields := ValidationError{}
	email := validateEmail(fields, rawEmail)
	if len(fields) > 0 {
		return fields
	}
	if _, err := service.repository.FindByEmail(ctx, email); err != nil {
		return err
	}
	return service.sendCode(ctx, email, clientIP, Pending{})
}

func (service *Service) sendCode(ctx context.Context, email, clientIP string, pending Pending) error {
	for _, limit := range []struct {
		key    string
		limit  int
		window time.Duration
	}{
		{"users:cooldown:" + digest(email), 1, codeCooldown},
		{"users:ip:" + digest(clientIP), ipCodeLimit, ipCodeWindow},
	} {
		allowed, err := service.store.Allow(ctx, limit.key, limit.limit, limit.window)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrRateLimited
		}
	}
	code, err := service.newCode()
	if err != nil {
		return err
	}
	pending.CodeHash = codeHash(email, code)
	if err := service.store.SaveCode(ctx, email, pending, codeTTL); err != nil {
		return err
	}
	return service.mailer.SendCode(ctx, email, code)
}

// Verify проверяет код и открывает сессию. remember - долгая сессия вместо обычной.
func (service *Service) Verify(ctx context.Context, rawEmail, code string, remember bool) (string, User, time.Duration, error) {
	fields := ValidationError{}
	email := validateEmail(fields, rawEmail)
	code = strings.TrimSpace(code)
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		fields["code"] = "Код - 6 цифр"
	}
	if len(fields) > 0 {
		return "", User{}, 0, fields
	}
	pending, err := service.store.CheckCode(ctx, email, codeHash(email, code), codeAttempts)
	if err != nil {
		return "", User{}, 0, err
	}

	var user User
	if pending.FirstName != "" {
		user, err = service.repository.Create(ctx, User{Email: email, FirstName: pending.FirstName, LastName: pending.LastName})
		if errors.Is(err, ErrEmailTaken) { // две регистрации одной почты подряд: вторая просто входит
			user, err = service.repository.FindByEmail(ctx, email)
		}
	} else {
		user, err = service.repository.FindByEmail(ctx, email)
	}
	if err != nil {
		return "", User{}, 0, err
	}

	ttl := service.sessionTTL
	if remember {
		ttl = RememberTTL
	}
	token, err := newToken()
	if err != nil {
		return "", User{}, 0, err
	}
	if err := service.store.CreateSession(ctx, token, user.ID, ttl); err != nil {
		return "", User{}, 0, err
	}
	return token, user, ttl, nil
}

func (service *Service) Authenticate(ctx context.Context, token string) (User, error) {
	if token == "" {
		return User{}, ErrNoSession
	}
	userID, err := service.store.SessionUser(ctx, token)
	if err != nil {
		return User{}, err
	}
	user, err := service.repository.FindByID(ctx, userID)
	if errors.Is(err, ErrUserNotFound) {
		_ = service.store.DeleteSession(ctx, token)
		return User{}, ErrNoSession
	}
	return user, err
}

func (service *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return service.store.DeleteSession(ctx, token)
}

func validateName(fields ValidationError, key, value string) string {
	value = strings.Join(strings.Fields(value), " ")
	switch {
	case value == "":
		fields[key] = "Обязательное поле"
	case utf8.RuneCountInString(value) > nameMaxRunes:
		fields[key] = fmt.Sprintf("Не более %d символов", nameMaxRunes)
	case strings.IndexFunc(value, func(r rune) bool { return !unicode.IsLetter(r) && !strings.ContainsRune(" -'", r) }) >= 0:
		fields[key] = "Только буквы, пробел, дефис и апостроф"
	}
	return value
}

func validateEmail(fields ValidationError, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		fields["email"] = "Обязательное поле"
		return value
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || !strings.Contains(value[strings.LastIndex(value, "@"):], ".") {
		fields["email"] = "Некорректный email"
	} else if utf8.RuneCountInString(value) > emailMaxRunes {
		fields["email"] = fmt.Sprintf("Не более %d символов", emailMaxRunes)
	}
	return value
}

func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("generate code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func newToken() (string, error) {
	value := make([]byte, sessionTokenLen)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func codeHash(email, code string) string { return digest(email + "\x00" + code) }

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
