package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/airsss993/histproject/backend/internal/modules/users"
	"github.com/airsss993/histproject/backend/internal/platform/database"
)

type siteRemoverStub struct{ archives, sites []string }

func (stub *siteRemoverStub) DeleteArchive(_ context.Context, archiveID string) error {
	stub.archives = append(stub.archives, archiveID)
	return nil
}

func (stub *siteRemoverStub) DeleteSite(_ context.Context, slug string) error {
	stub.sites = append(stub.sites, slug)
	return nil
}

func TestRetentionAndAccountDeletion(t *testing.T) {
	databaseURL := os.Getenv("TEST_APP_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_APP_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	prepareAppTestDatabase(t, ctx, databaseURL)
	databaseClient, err := database.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(databaseClient.Close)
	pool := databaseClient.Pool()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	// Две отклонённые заявки: старая и свежая, у старой есть журнал модерации.
	exec(`INSERT INTO submissions.submissions (id, title, description, event_date, event_type_code, email, telegram_username, archive_id, slug, status, updated_at)
		VALUES (1, 'old', 'd', '2020-01-01', 'military', 'a@example.test', '', 'arch-old', 'old-slug', 'rejected', NOW() - INTERVAL '91 days'),
		       (2, 'new', 'd', '2020-01-01', 'military', 'a@example.test', '', 'arch-new', 'new-slug', 'rejected', NOW() - INTERVAL '10 days'),
		       (3, 'pub', 'd', '2020-01-01', 'military', 'a@example.test', '', 'arch-pub', 'pub-slug', 'review', NOW() - INTERVAL '400 days')`)
	exec(`INSERT INTO submissions.audit_log (submission_id, admin_login, admin_role, action, created_at)
		VALUES (1, 'root', 'super_admin', 'rejected', NOW() - INTERVAL '91 days'),
		       (3, 'root', 'super_admin', 'reviewed', NOW() - INTERVAL '4 years'),
		       (3, 'root', 'super_admin', 'reviewed', NOW())`)
	if _, err := pool.Exec(ctx, `DELETE FROM submissions.audit_log`); err == nil {
		t.Fatal("audit log deletable outside retention")
	}

	files := &siteRemoverStub{}
	if err := NewRetention(pool, files, slog.New(slog.NewTextHandler(io.Discard, nil))).Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	var ids []int64
	rows, _ := pool.Query(ctx, `SELECT id FROM submissions.submissions ORDER BY id`)
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	if len(ids) != 2 || ids[0] != 2 || ids[1] != 3 {
		t.Fatalf("submissions left = %v, want [2 3]", ids)
	}
	if len(files.archives) != 1 || files.archives[0] != "arch-old" || files.sites[0] != "old-slug" {
		t.Fatalf("files removed = %#v", files)
	}
	var audit int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM submissions.audit_log`).Scan(&audit)
	if audit != 1 {
		t.Fatalf("audit rows = %d, want 1", audit)
	}

	// Согласие на распространение и удаление учётки.
	repository := users.NewRepository(pool)
	user, err := repository.Create(ctx, users.User{Email: "anna@example.test", FirstName: "Анна", LastName: "Петрова"}, users.RegistrationConsentVersion)
	if err != nil || user.ShowLastName {
		t.Fatalf("Create() = %#v, %v", user, err)
	}
	for _, enabled := range []bool{true, true, false, true} {
		if err := repository.SetDistribution(ctx, user.ID, enabled, users.DistributionConsentVersion); err != nil {
			t.Fatalf("SetDistribution(%v) error = %v", enabled, err)
		}
	}
	if found, err := repository.FindByID(ctx, user.ID); err != nil || !found.ShowLastName {
		t.Fatalf("FindByID() = %#v, %v", found, err)
	}
	var consents int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users.consents WHERE user_id = $1`, user.ID).Scan(&consents)
	if consents != 3 { // регистрация, выдача, повторная выдача после отзыва
		t.Fatalf("consent rows = %d, want 3", consents)
	}
	if err := repository.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM users.consents`).Scan(&consents)
	if _, err := repository.FindByID(ctx, user.ID); err != users.ErrUserNotFound || consents != 0 {
		t.Fatalf("after delete: err %v, consents %d", err, consents)
	}
}
