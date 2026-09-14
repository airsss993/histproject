package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestLoginChecksBcryptAndCreatesSession(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	credentials := &credentialReaderStub{
		adminID:      7,
		login:        "admin",
		passwordHash: string(passwordHash),
		role:         "admin",
	}
	sessions := &sessionStoreStub{}
	service := newService(credentials, sessions, 12*time.Hour, func() (string, error) {
		return "generated-token", nil
	})

	token, identity, err := service.Login(t.Context(), " admin ", "correct-password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if credentials.requestedLogin != "admin" {
		t.Fatalf("requested login = %q", credentials.requestedLogin)
	}
	if token != "generated-token" {
		t.Fatalf("token = %q", token)
	}
	if identity != (Identity{AdminID: 7, Login: "admin", Role: "admin"}) {
		t.Fatalf("identity = %#v", identity)
	}
	if sessions.createdToken != token || sessions.createdIdentity != identity || sessions.createdTTL != 12*time.Hour {
		t.Fatalf("created session = token:%q identity:%#v ttl:%s", sessions.createdToken, sessions.createdIdentity, sessions.createdTTL)
	}
}

func TestLoginReturnsSameErrorForMissingAdminAndWrongPassword(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}
	tests := []struct {
		name       string
		reader     *credentialReaderStub
		password   string
		wantLookup bool
	}{
		{
			name:       "missing admin",
			reader:     &credentialReaderStub{err: errors.New("not found")},
			password:   "correct-password",
			wantLookup: true,
		},
		{
			name: "wrong password",
			reader: &credentialReaderStub{
				adminID:      7,
				login:        "admin",
				passwordHash: string(passwordHash),
				role:         "admin",
			},
			password:   "wrong-password",
			wantLookup: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := newService(test.reader, &sessionStoreStub{}, time.Hour, func() (string, error) {
				return "unused", nil
			})

			_, _, err := service.Login(t.Context(), "admin", test.password)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestAuthenticateAndLogoutUseSessionStore(t *testing.T) {
	identity := Identity{AdminID: 7, Login: "admin", Role: "admin"}
	sessions := &sessionStoreStub{identity: identity}
	service := newService(&credentialReaderStub{}, sessions, time.Hour, func() (string, error) {
		return "unused", nil
	})

	got, err := service.Authenticate(t.Context(), "session-token")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if got != identity || sessions.getToken != "session-token" {
		t.Fatalf("Authenticate() = %#v, token %q", got, sessions.getToken)
	}

	if err := service.Logout(t.Context(), "session-token"); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if sessions.deletedToken != "session-token" {
		t.Fatalf("deleted token = %q", sessions.deletedToken)
	}
}

func TestNewSessionTokenIsOpaqueAndRandom(t *testing.T) {
	first, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken() error = %v", err)
	}
	second, err := newSessionToken()
	if err != nil {
		t.Fatalf("newSessionToken() second error = %v", err)
	}

	if first == second {
		t.Fatal("two session tokens are equal")
	}
	if len(first) < 40 || strings.ContainsAny(first, "+/=") {
		t.Fatalf("session token format = %q", first)
	}
}

type credentialReaderStub struct {
	adminID        int64
	login          string
	passwordHash   string
	role           string
	err            error
	requestedLogin string
}

func (stub *credentialReaderStub) FindCredentialsByLogin(_ context.Context, login string) (int64, string, string, string, error) {
	stub.requestedLogin = login
	return stub.adminID, stub.login, stub.passwordHash, stub.role, stub.err
}

type sessionStoreStub struct {
	createdToken    string
	createdIdentity Identity
	createdTTL      time.Duration
	createError     error
	getToken        string
	identity        Identity
	getError        error
	deletedToken    string
	deleteError     error
}

func (stub *sessionStoreStub) Create(_ context.Context, token string, identity Identity, ttl time.Duration) error {
	stub.createdToken = token
	stub.createdIdentity = identity
	stub.createdTTL = ttl
	return stub.createError
}

func (stub *sessionStoreStub) Get(_ context.Context, token string) (Identity, error) {
	stub.getToken = token
	return stub.identity, stub.getError
}

func (stub *sessionStoreStub) Delete(_ context.Context, token string) error {
	stub.deletedToken = token
	return stub.deleteError
}
