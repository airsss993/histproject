package router

import (
	"net/http"
	"strings"

	"github.com/airsss993/histproject-backend/internal/admin"
	"github.com/airsss993/histproject-backend/internal/config"
	"github.com/airsss993/histproject-backend/internal/objects"
	"github.com/airsss993/histproject-backend/internal/requests"
	"github.com/gin-gonic/gin"
)

// Handlers — все HTTP-хендлеры приложения.
type Handlers struct {
	Objects  *objects.Handler
	Requests *requests.Handler
	Admin    *admin.Handler
}

func New(cfg *config.Config, h Handlers, adminSvc *admin.Service) *gin.Engine {
	r := gin.Default()
	r.MaxMultipartMemory = 32 << 20 
	r.Use(corsMiddleware(cfg.CORS.AllowedOrigins))

	InitRoutes(r, h, adminSvc)

	return r
}

func InitRoutes(r *gin.Engine, h Handlers, adminSvc *admin.Service) {
	public := r.Group("/api")
	{
		public.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		})

		public.GET("objects/:id", h.Objects.GetObjectData)
		public.GET("objects", h.Objects.GetObjectsList)
		public.GET("sites/auth", h.Objects.SiteAuth)

		public.POST("requests", h.Requests.CreateRequest)

		// Публичные эндпоинты авторизации админов
		public.POST("admin/login", h.Admin.Login)
		public.POST("admin/refresh", h.Admin.Refresh)
	}

	// Защищённые эндпоинты админ-панели
	adminGroup := r.Group("/api/admin")
	adminGroup.Use(admin.JWTMiddleware(adminSvc))
	{
		adminGroup.GET("/ping", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"message": "pong", "adminId": c.GetInt("adminId"), "role": c.GetString("role")})
		})
		adminGroup.POST("/logout", h.Admin.Logout)
		adminGroup.GET("/preview", h.Admin.PreviewAuth)

		// Заявки
		adminGroup.GET("/requests", h.Admin.GetRequests)
		adminGroup.GET("/requests/:id", h.Admin.GetRequest)
		adminGroup.GET("/history", admin.RequireRole("super_admin"), h.Admin.GetRequestHistory)

		// Модерация заявок — любой авторизованный админ
		adminGroup.PATCH("/requests/:id/review", admin.RequireRole("admin", "super_admin"), h.Requests.ReviewRequest)
		adminGroup.PATCH("/requests/:id/publish", admin.RequireRole("admin", "super_admin"), h.Requests.PublishRequest)
		adminGroup.PATCH("/requests/:id/reject", admin.RequireRole("admin", "super_admin"), h.Requests.RejectRequest)

		// Управление объектами на карте — любой авторизованный админ
		adminGroup.DELETE("/requests/:id/object", admin.RequireRole("admin", "super_admin"), h.Objects.DeleteObject)
		adminGroup.PATCH("/requests/:id/object/coordinates", admin.RequireRole("admin", "super_admin"), h.Objects.UpdateCoordinates)

		// Управление админами — только super_admin
		adminGroup.GET("/admins", admin.RequireRole("super_admin"), h.Admin.ListAdmins)
		adminGroup.POST("/admins", admin.RequireRole("super_admin"), h.Admin.CreateAdmin)
		adminGroup.DELETE("/admins/:id", admin.RequireRole("super_admin"), h.Admin.DeleteAdmin)
		adminGroup.PATCH("/admins/:id/role", admin.RequireRole("super_admin"), h.Admin.UpdateAdminRole)
	}
}

func corsMiddleware(allowedOrigins string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if origin != "" && strings.Contains(allowedOrigins, origin) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		} else if allowedOrigins == "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
