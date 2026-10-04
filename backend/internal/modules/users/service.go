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
	codeTTL      = 10 * time.Minute
	codeAttempts = 5
	codeCooldown = time.Minute
	ipCodeLimit  = 10
	ipCodeWindow = 10 * time.Minute
	RememberTTL  = 30 * 24 * time.Hour
	nameMaxRunes = 50
	// Версии текстов согласий на /consent/registration и /consent/distribution. Поменялся текст - новая дата.
	RegistrationConsentVersion = "2026-09-28"
	DistributionConsentVersion = "2026-10-04"
	emailMaxRunes              = 254
	sessionTokenLen            = 32
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
	PublicID  string    `json:"publicId"`
	Email     string    `json:"email"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	CreatedAt time.Time `json:"createdAt"`
	// ShowLastName - действует согласие на распространение: посторонним видна фамилия целиком.
	ShowLastName bool `json:"showLastName"`
}

// PublicLastName - фамилия для посторонних: без согласия на распространение только первая буква.
func (user User) PublicLastName() string {
	if user.ShowLastName || user.LastName == "" {
		return user.LastName
	}
	first, _ := utf8.DecodeRuneInString(user.LastName)
	return string(first) + "."
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
	FindByPublicID(context.Context, string) (User, error)
	Create(ctx context.Context, user User, consentVersion string) (User, error)
	SetDistribution(ctx context.Context, userID int64, enabled bool, version string) error
	Delete(ctx context.Context, userID int64) error
}

type Store interface {
	SaveCode(context.Context, string, Pending, time.Duration) error
	// CheckCode сверяет хеш и удаляет код при успехе. ErrInvalidCode - не совпал, ErrCodeExpired - нет кода или попытки кончились.
	CheckCode(context.Context, string, string, int) (Pending, error)
	Allow(context.Context, string, int, time.Duration) (bool, error)
	ResetLimit(context.Context, string) error
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
	works      Works
	sessionTTL time.Duration
	newCode    func() (string, error)
}

func NewService(repository Repository, store Store, mailer Mailer, works Works, sessionTTL time.Duration) *Service {
	return &Service{repository: repository, store: store, mailer: mailer, works: works, sessionTTL: sessionTTL, newCode: newCode}
}

type RegisterInput struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	// Consent - галочка согласия на обработку. Без неё код не отправляется.
	Consent bool `json:"consent"`
}

// Register шлёт код на почту. Учётка появится только после Verify: неподтверждённых записей в базе нет.
func (service *Service) Register(ctx context.Context, input RegisterInput, clientKey string) error {
	fields := ValidationError{}
	firstName := validateName(fields, "firstName", input.FirstName)
	lastName := validateName(fields, "lastName", input.LastName)
	email := validateEmail(fields, input.Email)
	if !input.Consent {
		fields["consent"] = "Нужно согласие на обработку данных"
	}
	if len(fields) > 0 {
		return fields
	}
	if _, err := service.repository.FindByEmail(ctx, email); err == nil {
		return ErrEmailTaken
	} else if !errors.Is(err, ErrUserNotFound) {
		return err
	}
	return service.sendCode(ctx, email, clientKey, Pending{FirstName: firstName, LastName: lastName})
}

func (service *Service) RequestLogin(ctx context.Context, rawEmail, clientKey string) error {
	fields := ValidationError{}
	email := validateEmail(fields, rawEmail)
	if len(fields) > 0 {
		return fields
	}
	if _, err := service.repository.FindByEmail(ctx, email); err != nil {
		return err
	}
	return service.sendCode(ctx, email, clientKey, Pending{})
}

func (service *Service) sendCode(ctx context.Context, email, clientKey string, pending Pending) error {
	cooldownKey := "users:cooldown:" + digest(email)
	for _, limit := range []struct {
		key    string
		limit  int
		window time.Duration
	}{
		{cooldownKey, 1, codeCooldown},
		{"users:ip:" + clientKey, ipCodeLimit, ipCodeWindow},
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
	if err := service.mailer.SendCode(ctx, email, code); err != nil {
		// Письмо не ушло - минута ожидания не нужна, иначе повтор получит 429 вместо настоящей ошибки.
		_ = service.store.ResetLimit(ctx, cooldownKey)
		return err
	}
	return nil
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
		user, err = service.repository.Create(ctx, User{Email: email, FirstName: pending.FirstName, LastName: pending.LastName}, RegistrationConsentVersion)
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

// SetDistribution включает или выключает показ фамилии целиком и отдаёт обновлённую учётку.
func (service *Service) SetDistribution(ctx context.Context, token string, enabled bool) (User, error) {
	user, err := service.Authenticate(ctx, token)
	if err != nil {
		return User{}, err
	}
	if err := service.repository.SetDistribution(ctx, user.ID, enabled, DistributionConsentVersion); err != nil {
		return User{}, err
	}
	user.ShowLastName = enabled
	return user, nil
}

// DeleteAccount удаляет учётку владельца сессии. Остальные его сессии отпадут сами:
// Authenticate не найдёт пользователя и сотрёт сессию.
func (service *Service) DeleteAccount(ctx context.Context, token string) error {
	user, err := service.Authenticate(ctx, token)
	if err != nil {
		return err
	}
	if err := service.repository.Delete(ctx, user.ID); err != nil {
		return err
	}
	return service.store.DeleteSession(ctx, token)
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
