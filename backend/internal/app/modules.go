package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
	"github.com/airsss993/histproject/backend/internal/modules/auth"
	"github.com/airsss993/histproject/backend/internal/modules/events"
	"github.com/airsss993/histproject/backend/internal/modules/submissions"
	"github.com/airsss993/histproject/backend/internal/modules/users"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/database"
	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
	platformqueue "github.com/airsss993/histproject/backend/internal/platform/queue"
	"github.com/airsss993/histproject/backend/internal/platform/redisstore"
	"github.com/airsss993/histproject/backend/internal/platform/storage"
)

type Modules struct {
	auth        *auth.Handler
	admins      *admins.Handler
	events      *events.Handler
	submissions *submissions.Handler
	users       *users.Handler
	rateLimit   httpserver.ClientRateLimiter
}

func NewModules(cfg config.Config, logger *slog.Logger, databaseClient *database.Database, redisClient *redisstore.Store, storageClient *storage.MinIO) Modules {
	adminRepository := admins.NewRepository(databaseClient.Pool())
	adminService := admins.NewService(adminRepository)
	authHandler := auth.NewHandler(
		auth.NewService(adminRepository, auth.NewSessionStore(redisClient.Client()), cfg.SessionTTL),
		auth.NewLoginLimiter(redisClient.Client()),
		logger,
		cfg.CookieDomain,
		cfg.SessionTTL,
	)
	adminHandler := admins.NewHandler(adminService, logger, func(ctx context.Context) (admins.Actor, bool) {
		identity, ok := auth.IdentityFromContext(ctx)
		return admins.Actor{AdminID: identity.AdminID, Role: admins.Role(identity.Role)}, ok
	})
	eventService := events.NewService(events.NewRepository(databaseClient.Pool()))
	eventHandler := events.NewHandler(eventService, logger)
	submissionRepository := submissions.NewRepository(databaseClient.Pool())
	queueClient := platformqueue.NewClient(redisClient.Client())
	submissionService := submissions.NewService(
		submissionRepository,
		submissionEventPublisher{service: eventService},
		storageClient,
		queueClient,
		NewSubmissionNotifier(queueClient),
		cfg.SitePublicBaseURL,
	)
	submissionHandler := submissions.NewHandler(
		submissionService,
		submissions.NewCreateLimiter(redisClient.Client()),
		logger,
		func(ctx context.Context) (submissions.Actor, bool) {
			identity, ok := auth.IdentityFromContext(ctx)
			return submissions.Actor{Login: identity.Login, Role: identity.Role}, ok
		},
		cfg.MaxUploadBytes,
	)
	userHandler := users.NewHandler(
		users.NewService(
			users.NewRepository(databaseClient.Pool()), users.NewStore(redisClient.Client()),
			NewCodeMailer(cfg, logger), authorWorks{service: submissionService}, cfg.SessionTTL,
		),
		logger,
		cfg.CookieDomain,
	)
	return Modules{auth: authHandler, admins: adminHandler, events: eventHandler, submissions: submissionHandler, users: userHandler, rateLimit: redisClient.Allow}
}

func (modules Modules) Register(mux *http.ServeMux) {
	if modules.auth == nil || modules.admins == nil || modules.events == nil || modules.submissions == nil {
		return
	}
	adminOnly := func(next http.Handler) http.Handler {
		return modules.auth.Authenticate(auth.RequireRole(string(admins.RoleAdmin), string(admins.RoleSuperAdmin))(next))
	}
	superAdminOnly := func(next http.Handler) http.Handler {
		return modules.auth.Authenticate(auth.RequireRole(string(admins.RoleSuperAdmin))(next))
	}
	modules.auth.Register(mux)
	modules.admins.Register(mux, superAdminOnly)
	modules.events.Register(mux, adminOnly)
	modules.submissions.Register(mux, adminOnly, superAdminOnly)
	if modules.users != nil {
		modules.users.Register(mux)
	}
}

// authorWorks отдаёт профилю заявки автора: модуль users не ходит в таблицы submissions сам.
type authorWorks struct {
	service *submissions.Service
}

func (works authorWorks) ByAuthor(ctx context.Context, email string) ([]users.Work, error) {
	found, err := works.service.ByAuthor(ctx, email)
	if err != nil {
		return nil, err
	}
	items := make([]users.Work, 0, len(found))
	for _, submission := range found {
		items = append(items, users.Work{
			PublicID: submission.PublicID, Title: submission.Title, EventDate: submission.EventDate.Format(time.DateOnly),
			EventType: submission.EventType, Status: string(submission.Status), SiteURL: submission.SiteURL,
			ScreenshotURL: submission.ScreenshotURL, Comment: submission.AdminComment, CreatedAt: submission.CreatedAt,
		})
	}
	return items, nil
}

type submissionEventPublisher struct {
	service *events.Service
}

func (publisher submissionEventPublisher) Publish(ctx context.Context, snapshot submissions.EventSnapshot) error {
	_, err := publisher.service.Publish(ctx, events.Snapshot{
		SubmissionID: snapshot.SubmissionID, Title: snapshot.Title, Description: snapshot.Description,
		Latitude: snapshot.Latitude, Longitude: snapshot.Longitude, EventDate: snapshot.EventDate,
		EventType: snapshot.EventType, SiteURL: snapshot.SiteURL, PreviewImageURL: snapshot.PreviewImageURL,
	})
	return err
}
