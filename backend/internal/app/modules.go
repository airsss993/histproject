package app

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
	"github.com/airsss993/histproject/backend/internal/modules/auth"
	"github.com/airsss993/histproject/backend/internal/platform/config"
	"github.com/airsss993/histproject/backend/internal/platform/database"
	"github.com/airsss993/histproject/backend/internal/platform/redisstore"
)

type Modules struct {
	auth   *auth.Handler
	admins *admins.Handler
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
	return Modules{auth: authHandler, admins: adminHandler}
}

func (modules Modules) Register(mux *http.ServeMux) {
	if modules.auth == nil || modules.admins == nil {
		return
	}
	modules.auth.Register(mux)
	modules.admins.Register(mux, func(next http.Handler) http.Handler {
		return modules.auth.Authenticate(auth.RequireRole(string(admins.RoleSuperAdmin))(next))
	})
}
