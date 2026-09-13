package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
	"github.com/airsss993/histproject/backend/internal/modules/auth"
	"github.com/airsss993/histproject/backend/internal/modules/events"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/database"
	"github.com/airsss993/histproject/backend/internal/platform/redisstore"
)

type Modules struct {
	auth   *auth.Handler
	admins *admins.Handler
	events *events.Handler
}

func NewModules(cfg config.Config, logger *slog.Logger, databaseClient *database.Database, redisClient *redisstore.Store) Modules {
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
	eventHandler := events.NewHandler(events.NewService(events.NewRepository(databaseClient.Pool())), logger)
	return Modules{auth: authHandler, admins: adminHandler, events: eventHandler}
}

func (modules Modules) Register(mux *http.ServeMux) {
	if modules.auth == nil || modules.admins == nil || modules.events == nil {
		return
	}
	modules.auth.Register(mux)
	modules.admins.Register(mux, func(next http.Handler) http.Handler {
		return modules.auth.Authenticate(auth.RequireRole(string(admins.RoleSuperAdmin))(next))
	})
	modules.events.Register(mux, func(next http.Handler) http.Handler {
		return modules.auth.Authenticate(auth.RequireRole(string(admins.RoleAdmin), string(admins.RoleSuperAdmin))(next))
	})
}
