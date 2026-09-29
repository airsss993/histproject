package users

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRegisterVerifyLoginFlow(t *testing.T) {
	ctx := t.Context()
	repo := &memoryRepository{}
	store := &memoryStore{codes: map[string]Pending{}, counts: map[string]int{}, sessions: map[string]int64{}}
	mailer := &lastCode{}
	service := NewService(repo, store, mailer, nil, 12*time.Hour)

	var fields ValidationError
	if err := service.Register(ctx, RegisterInput{FirstName: " ", LastName: "Ivanov1", Email: "bad"}, "ip"); !errors.As(err, &fields) || len(fields) != 4 {
		t.Fatalf("invalid register error = %v", err)
	}
	if err := service.Register(ctx, RegisterInput{FirstName: " Анна  Мария ", LastName: "Петрова-Водкина", Email: " Anna@Example.RU ", Consent: true}, "ip"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if mailer.email != "anna@example.ru" || len(mailer.code) != 6 {
		t.Fatalf("mail = %q %q", mailer.email, mailer.code)
	}
	if err := service.Register(ctx, RegisterInput{FirstName: "А", LastName: "Б", Email: "anna@example.ru", Consent: true}, "ip"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("second code within cooldown error = %v, want rate limited", err)
	}

	wrong := "000000"
	if mailer.code == wrong {
		wrong = "111111"
	}
	if _, _, _, err := service.Verify(ctx, "anna@example.ru", wrong, false); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("wrong code error = %v", err)
	}
	token, user, ttl, err := service.Verify(ctx, "ANNA@example.ru", mailer.code, false)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if user.FirstName != "Анна Мария" || user.LastName != "Петрова-Водкина" || ttl != 12*time.Hour {
		t.Fatalf("user = %#v ttl = %s", user, ttl)
	}
	if _, _, _, err := service.Verify(ctx, "anna@example.ru", mailer.code, false); !errors.Is(err, ErrCodeExpired) {
		t.Fatalf("reused code error = %v, want expired", err)
	}
	if me, err := service.Authenticate(ctx, token); err != nil || me.ID != user.ID {
		t.Fatalf("Authenticate() = %#v, %v", me, err)
	}

	if err := service.Register(ctx, RegisterInput{FirstName: "А", LastName: "Б", Email: "anna@example.ru", Consent: true}, "ip2"); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("register taken error = %v", err)
	}
	if err := service.RequestLogin(ctx, "nobody@example.ru", "ip"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("login unknown error = %v", err)
	}
	delete(store.counts, "users:cooldown:"+digest("anna@example.ru"))
	if err := service.RequestLogin(ctx, "anna@example.ru", "ip"); err != nil {
		t.Fatalf("RequestLogin() error = %v", err)
	}
	_, again, ttl, err := service.Verify(ctx, "anna@example.ru", mailer.code, true)
	if err != nil || again.ID != user.ID || ttl != RememberTTL || len(repo.users) != 1 {
		t.Fatalf("login verify = %#v ttl %s err %v users %d", again, ttl, err, len(repo.users))
	}

	if err := service.Logout(ctx, token); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := service.Authenticate(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Fatalf("after logout error = %v", err)
	}
}

type memoryRepository struct{ users []User }

func (r *memoryRepository) SetDistribution(_ context.Context, userID int64, enabled bool, _ string) error {
	for i := range r.users {
		if r.users[i].ID == userID {
			r.users[i].ShowLastName = enabled
		}
	}
	return nil
}

func (r *memoryRepository) Delete(_ context.Context, userID int64) error {
	for i, u := range r.users {
		if u.ID == userID {
			r.users = append(r.users[:i], r.users[i+1:]...)
		}
	}
	return nil
}

func (r *memoryRepository) FindByEmail(_ context.Context, email string) (User, error) {
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (r *memoryRepository) FindByID(_ context.Context, id int64) (User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (r *memoryRepository) FindByPublicID(_ context.Context, code string) (User, error) {
	for _, u := range r.users {
		if u.PublicID == "RHU-"+code {
			return u, nil
		}
	}
	return User{}, ErrUserNotFound
}

func (r *memoryRepository) Create(ctx context.Context, user User, _ string) (User, error) {
	if _, err := r.FindByEmail(ctx, user.Email); err == nil {
		return User{}, ErrEmailTaken
	}
	user.ID = int64(len(r.users) + 1)
	r.users = append(r.users, user)
	return user, nil
}

type memoryStore struct {
	codes    map[string]Pending
	counts   map[string]int
	sessions map[string]int64
}

func (s *memoryStore) SaveCode(_ context.Context, email string, p Pending, _ time.Duration) error {
	s.codes[email] = p
	return nil
}

func (s *memoryStore) CheckCode(_ context.Context, email, hash string, _ int) (Pending, error) {
	p, ok := s.codes[email]
	if !ok {
		return Pending{}, ErrCodeExpired
	}
	if p.CodeHash != hash {
		return Pending{}, ErrInvalidCode
	}
	delete(s.codes, email)
	return p, nil
}

func (s *memoryStore) Allow(_ context.Context, key string, limit int, _ time.Duration) (bool, error) {
	s.counts[key]++
	return s.counts[key] <= limit, nil
}

func (s *memoryStore) ResetLimit(_ context.Context, key string) error {
	delete(s.counts, key)
	return nil
}

func (s *memoryStore) CreateSession(_ context.Context, token string, id int64, _ time.Duration) error {
	s.sessions[token] = id
	return nil
}

func (s *memoryStore) SessionUser(_ context.Context, token string) (int64, error) {
	id, ok := s.sessions[token]
	if !ok {
		return 0, ErrNoSession
	}
	return id, nil
}

func (s *memoryStore) DeleteSession(_ context.Context, token string) error {
	delete(s.sessions, token)
	return nil
}

type lastCode struct {
	email, code string
	err         error
}

func (m *lastCode) SendCode(_ context.Context, email, code string) error {
	m.email, m.code = email, code
	return m.err
}

func TestFailedMailDoesNotBurnCooldown(t *testing.T) {
	ctx := t.Context()
	repo := &memoryRepository{users: []User{{ID: 1, Email: "anna@example.ru"}}}
	store := &memoryStore{codes: map[string]Pending{}, counts: map[string]int{}, sessions: map[string]int64{}}
	mailer := &lastCode{err: errors.New("postbox down")}
	service := NewService(repo, store, mailer, nil, time.Hour)

	if err := service.RequestLogin(ctx, "anna@example.ru", "ip"); !errors.Is(err, mailer.err) {
		t.Fatalf("RequestLogin() error = %v, want mailer error", err)
	}
	mailer.err = nil
	if err := service.RequestLogin(ctx, "anna@example.ru", "ip"); err != nil {
		t.Fatalf("retry after failed mail error = %v, want nil", err)
	}
	if err := service.RequestLogin(ctx, "anna@example.ru", "ip"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("third request error = %v, want rate limited", err)
	}
}

type worksStub struct {
	email    string
	viewerID int64
	works    []Work
}

func (stub *worksStub) ByAuthor(_ context.Context, email string, viewerID int64) ([]Work, error) {
	stub.email, stub.viewerID = email, viewerID
	return stub.works, nil
}

func TestProfileHidesUnpublishedFromGuest(t *testing.T) {
	ctx := t.Context()
	repo := &memoryRepository{users: []User{{ID: 1, PublicID: "RHU-7K3F10", Email: "anna@example.ru", FirstName: "Анна", LastName: "Петрова"}}}
	store := &memoryStore{codes: map[string]Pending{}, counts: map[string]int{}, sessions: map[string]int64{"token": 1}}
	works := &worksStub{works: []Work{
		{PublicID: "RH-5", Status: "published", Comment: "опубликовано", Stats: &WorkStats{Views: 3}},
		{PublicID: "RH-6", Status: "rejected", Comment: "архив без index.html"},
	}}
	service := NewService(repo, store, &lastCode{}, works, 12*time.Hour)

	owner, err := service.Profile(ctx, "RHU-7K3F10", "token")
	if err != nil {
		t.Fatalf("Profile() owner error = %v", err)
	}
	if !owner.Owner || len(owner.Works) != 2 || owner.Works[1].Comment == "" || owner.Works[0].Stats == nil {
		t.Fatalf("owner profile = %+v, want both works with comments and stats", owner)
	}
	if works.email != "anna@example.ru" || works.viewerID != 1 {
		t.Fatalf("works asked for %q by viewer %d", works.email, works.viewerID)
	}

	guest, err := service.Profile(ctx, "rhu-7k3fio", "")
	if err != nil {
		t.Fatalf("Profile() guest error = %v", err)
	}
	if guest.Owner || len(guest.Works) != 1 || guest.Works[0].PublicID != "RH-5" || guest.Works[0].Comment != "" || guest.Works[0].Stats != nil {
		t.Fatalf("guest profile = %+v, want one published work without comment and stats", guest)
	}
	if works.viewerID != 0 {
		t.Fatalf("guest works asked by viewer %d, want 0", works.viewerID)
	}

	for _, id := range []string{"RHU-000000", "17", "RHU-7K3F1", "RH-7K3F10", "RHU-7K3FU0"} {
		if _, err := service.Profile(ctx, id, ""); !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("Profile(%q) error = %v, want ErrUserNotFound", id, err)
		}
	}
}

func TestPublicLastNameNeedsDistributionConsent(t *testing.T) {
	for _, tc := range []struct {
		user User
		want string
	}{
		{User{LastName: "Ёлкина"}, "Ё."},
		{User{LastName: "Ёлкина", ShowLastName: true}, "Ёлкина"},
		{User{LastName: ""}, ""},
	} {
		if got := tc.user.PublicLastName(); got != tc.want {
			t.Fatalf("PublicLastName(%#v) = %q, want %q", tc.user, got, tc.want)
		}
	}
}

func TestDistributionAndDeleteAccount(t *testing.T) {
	ctx := t.Context()
	repo := &memoryRepository{users: []User{{ID: 1, PublicID: "RHU-7K3F9Q", Email: "anna@example.ru", FirstName: "Анна", LastName: "Петрова"}}}
	store := &memoryStore{codes: map[string]Pending{}, counts: map[string]int{}, sessions: map[string]int64{"token": 1, "other": 1}}
	service := NewService(repo, store, &lastCode{}, nil, time.Hour)

	if _, err := service.SetDistribution(ctx, "", true); !errors.Is(err, ErrNoSession) {
		t.Fatalf("guest SetDistribution() error = %v", err)
	}
	user, err := service.SetDistribution(ctx, "token", true)
	if err != nil || !user.ShowLastName {
		t.Fatalf("SetDistribution() = %#v, %v", user, err)
	}
	if profile, err := service.Profile(ctx, "RHU-7K3F9Q", ""); err != nil || profile.User.LastName != "Петрова" {
		t.Fatalf("profile with consent = %#v, %v", profile.User, err)
	}
	if _, err := service.SetDistribution(ctx, "token", false); err != nil {
		t.Fatalf("revoke error = %v", err)
	}
	if profile, err := service.Profile(ctx, "RHU-7K3F9Q", ""); err != nil || profile.User.LastName != "П." {
		t.Fatalf("profile without consent = %#v, %v", profile.User, err)
	}

	if err := service.DeleteAccount(ctx, "token"); err != nil {
		t.Fatalf("DeleteAccount() error = %v", err)
	}
	if len(repo.users) != 0 {
		t.Fatalf("users after delete = %#v", repo.users)
	}
	if _, err := service.Authenticate(ctx, "other"); !errors.Is(err, ErrNoSession) {
		t.Fatalf("other session after delete error = %v", err)
	}
}
