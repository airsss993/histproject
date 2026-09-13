package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
	"github.com/airsss993/histproject/backend/internal/modules/auth"
	"github.com/airsss993/histproject/backend/internal/modules/events"
	"github.com/airsss993/histproject/backend/internal/modules/submissions"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/database"
	platformqueue "github.com/airsss993/histproject/backend/internal/platform/queue"
	"github.com/airsss993/histproject/backend/internal/platform/redisstore"
	"github.com/airsss993/histproject/backend/internal/platform/storage"
)

type Modules struct {
	auth        *auth.Handler
	admins      *admins.Handler
	events      *events.Handler
	submissions *submissions.Handler
}

func NewModules(cfg config.Config, logger *slog.Logger, databaseClient *database.Database, redisClient *redisstore.Store, storageClient *storage.MinIO) Modules {
	adminRepository := admins.NewRepository(databaseClient.Pool())
	authHandler := auth.NewHandler(
		auth.NewService(adminRepository, auth.NewSessionStore(redisClient.Client()), cfg.SessionTTL),
		auth.NewLoginLimiter(redisClient.Client()),
		logger,
		cfg.CookieDomain,
		cfg.SessionTTL,
	)
	adminHandler := admins.NewHandler(admins.NewService(adminRepository), logger, func(ctx context.Context) (admins.Actor, bool) {
		identity, ok := auth.IdentityFromContext(ctx)
		return admins.Actor{AdminID: identity.AdminID, Role: admins.Role(identity.Role)}, ok
	})
	eventService := events.NewService(events.NewRepository(databaseClient.Pool()))
	eventHandler := events.NewHandler(eventService, logger)
	submissionRepository := submissions.NewRepository(databaseClient.Pool())
	submissionService := submissions.NewService(
		submissionRepository,
		submissionEventPublisher{service: eventService},
		storageClient,
		platformqueue.NewClient(redisClient.Client()),
		nil,
		cfg.MaxUploadBytes,
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
	return Modules{auth: authHandler, admins: adminHandler, events: eventHandler, submissions: submissionHandler}
}

func (modules Modules) Register(mux *http.ServeMux) {
	if modules.auth == nil || modules.admins == nil || modules.events == nil || modules.submissions == nil {
		return
	}
	modules.auth.Register(mux)
	modules.admins.Register(mux, func(next http.Handler) http.Handler {
		return modules.auth.Authenticate(auth.RequireRole(string(admins.RoleSuperAdmin))(next))
	})
	modules.events.Register(mux, func(next http.Handler) http.Handler {
		return modules.auth.Authenticate(auth.RequireRole(string(admins.RoleAdmin), string(admins.RoleSuperAdmin))(next))
	})
	modules.submissions.Register(
		mux,
		func(next http.Handler) http.Handler {
			return modules.auth.Authenticate(auth.RequireRole(string(admins.RoleAdmin), string(admins.RoleSuperAdmin))(next))
		},
		func(next http.Handler) http.Handler {
			return modules.auth.Authenticate(auth.RequireRole(string(admins.RoleSuperAdmin))(next))
		},
	)
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
