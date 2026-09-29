package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/airsss993/histproject/backend/internal/modules/admins"
	"github.com/airsss993/histproject/backend/internal/modules/auth"
	"github.com/airsss993/histproject/backend/internal/modules/events"
	"github.com/airsss993/histproject/backend/internal/modules/notifications"
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
	bot         *notifications.BotHandler
	mail        *notifications.MailHandler
	geocoder    *Geocoder
	rateLimit   httpserver.ClientRateLimiter
}

func NewModules(cfg config.Config, logger *slog.Logger, databaseClient *database.Database, redisClient *redisstore.Store, storageClient *storage.MinIO) Modules {
	adminRepository := admins.NewRepository(databaseClient.Pool())
	adminService := admins.NewService(adminRepository)
	submissionRepository := submissions.NewRepository(databaseClient.Pool())
	userRepository := users.NewRepository(databaseClient.Pool())
	eventService := events.NewService(events.NewRepository(databaseClient.Pool()), eventAuthors{submissions: submissionRepository, users: userRepository})
	queueClient := platformqueue.NewClient(redisClient.Client())
	submissionService := submissions.NewService(
		submissionRepository,
		submissionEventPublisher{service: eventService},
		storageClient,
		queueClient,
		NewSubmissionNotifier(queueClient),
		cfg.SitePublicBaseURL,
	)
	notificationRepository := notifications.NewRepository(databaseClient.Pool())
	userService := users.NewService(
		userRepository, users.NewStore(redisClient.Client()),
		NewCodeMailer(cfg, logger, notificationRepository), authorWorks{submissions: submissionService, events: eventService}, cfg.SessionTTL,
	)
	eventHandler := events.NewHandler(eventService, logger, eventViewer(userService))
	authHandler := auth.NewHandler(auth.NewService(adminRepository), currentUserEmail(userService), logger)
	adminHandler := admins.NewHandler(adminService, logger, func(ctx context.Context) (admins.Actor, bool) {
		identity, ok := auth.IdentityFromContext(ctx)
		return admins.Actor{AdminID: identity.AdminID, Role: admins.Role(identity.Role)}, ok
	})
	submissionHandler := submissions.NewHandler(
		submissionService,
		submissions.NewCreateLimiter(redisClient.Client()),
		logger,
		func(ctx context.Context) (submissions.Actor, bool) {
			identity, ok := auth.IdentityFromContext(ctx)
			return submissions.Actor{Login: identity.Login, Role: identity.Role}, ok
		},
		visitorEmail(userService),
		cfg.MaxUploadBytes,
	)
	userHandler := users.NewHandler(userService, logger, cfg.CookieDomain)
	botHandler := notifications.NewBotHandler(notifications.NewBot(notificationRepository, cfg.NotificationTimeout), logger)
	return Modules{
		auth: authHandler, admins: adminHandler, events: eventHandler, submissions: submissionHandler, users: userHandler,
		bot: botHandler, mail: NewMailHandler(cfg, logger, notificationRepository), geocoder: NewGeocoder(logger),
		rateLimit: redisClient.Allow,
	}
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
	if modules.bot != nil {
		modules.bot.Register(mux, superAdminOnly)
	}
	if modules.mail != nil {
		modules.mail.Register(mux, superAdminOnly)
	}
	if modules.geocoder != nil {
		modules.geocoder.Register(mux)
	}
}

func currentUserEmail(service *users.Service) auth.UserEmail {
	return func(r *http.Request) (string, error) {
		token := ""
		if cookie, err := r.Cookie(users.SessionCookieName); err == nil {
			token = cookie.Value
		}
		user, err := service.Authenticate(r.Context(), token)
		if errors.Is(err, users.ErrNoSession) {
			return "", auth.ErrUnauthenticated
		}
		return user.Email, err
	}
}

// visitorEmail - автор заявки из сессии посетителя: email берётся из аккаунта, а не из формы.
func visitorEmail(service *users.Service) submissions.AuthorReader {
	return func(r *http.Request) (string, error) {
		token := ""
		if cookie, err := r.Cookie(users.SessionCookieName); err == nil {
			token = cookie.Value
		}
		user, err := service.Authenticate(r.Context(), token)
		if errors.Is(err, users.ErrNoSession) {
			return "", submissions.ErrUnauthorized
		}
		return user.Email, err
	}
}

// eventViewer - смотрящий для звёзд и просмотров. Гость - нулевой Viewer без ошибки.
func eventViewer(service *users.Service) events.ViewerReader {
	return func(r *http.Request) (events.Viewer, error) {
		token := ""
		if cookie, err := r.Cookie(users.SessionCookieName); err == nil {
			token = cookie.Value
		}
		user, err := service.Authenticate(r.Context(), token)
		if errors.Is(err, users.ErrNoSession) {
			return events.Viewer{}, nil
		}
		if err != nil {
			return events.Viewer{}, err
		}
		return events.Viewer{UserID: user.ID, Email: user.Email}, nil
	}
}

// eventAuthors - автор события для events: почта заявки отличает автора от посетителя,
// учётка с этой почтой даёт имя и ссылку на профиль под сайтом.
type eventAuthors struct {
	submissions *submissions.Repository
	users       *users.PostgresRepository
}

func (authors eventAuthors) Author(ctx context.Context, submissionID int64) (events.Author, error) {
	submission, err := authors.submissions.Get(ctx, submissionID)
	if errors.Is(err, submissions.ErrSubmissionNotFound) {
		return events.Author{}, nil
	}
	if err != nil {
		return events.Author{}, err
	}
	author := events.Author{Email: submission.Email}
	user, err := authors.users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(submission.Email)))
	if errors.Is(err, users.ErrUserNotFound) {
		return author, nil
	}
	if err != nil {
		return events.Author{}, err
	}
	author.PublicID, author.FirstName, author.LastName = user.PublicID, user.FirstName, user.PublicLastName()
	return author, nil
}

// authorWorks отдаёт профилю заявки автора со звёздами и просмотрами: модуль users не ходит
// в таблицы submissions и events сам.
type authorWorks struct {
	submissions *submissions.Service
	events      *events.Service
}

func (works authorWorks) ByAuthor(ctx context.Context, email string, viewerID int64) ([]users.Work, error) {
	found, err := works.submissions.ByAuthor(ctx, email)
	if err != nil {
		return nil, err
	}
	published := make([]int64, 0, len(found))
	for _, submission := range found {
		if submission.Status == submissions.StatusPublished {
			published = append(published, submission.ID)
		}
	}
	summaries, err := works.events.Summaries(ctx, published, viewerID)
	if err != nil {
		return nil, err
	}
	items := make([]users.Work, 0, len(found))
	for _, submission := range found {
		work := users.Work{
			PublicID: submission.PublicID, Title: submission.Title, EventDate: submission.EventDate.Format(time.DateOnly),
			EventType: submission.EventType, Status: string(submission.Status), SiteURL: submission.SiteURL,
			ScreenshotURL: submission.ScreenshotURL, Comment: submission.AdminComment, CreatedAt: submission.CreatedAt,
		}
		if summary, ok := summaries[submission.ID]; ok {
			work.EventID = &summary.EventID
			work.Stars, work.Starred, work.Visitors = summary.Stars, summary.Starred, summary.Visitors
			work.Stats = &users.WorkStats{Views: summary.Views, Visitors: summary.Visitors, ActiveSeconds: summary.ActiveSeconds}
		}
		items = append(items, work)
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
