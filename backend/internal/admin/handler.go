package admin

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler — HTTP-обработчики аутентификации администраторов.
type Handler struct {
	svc          *Service
	cookieDomain string
}

// NewHandler создаёт handler администраторов.
func NewHandler(svc *Service, cookieDomain string) *Handler {
	return &Handler{svc: svc, cookieDomain: cookieDomain}
}

// Login авторизует администратора и устанавливает токены в куки.
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ошибка валидации: " + err.Error()})
		return
	}

	tokens, err := h.svc.Login(c.Request.Context(), req.Login, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		return
	}

	h.setTokenCookies(c, tokens)
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Refresh читает refresh-токен из куки и выдаёт новую пару токенов.
func (h *Handler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Отсутствует refresh-токен"})
		return
	}

	tokens, err := h.svc.Refresh(c.Request.Context(), refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": err.Error()})
		return
	}

	h.setTokenCookies(c, tokens)
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// Logout удаляет сессию и очищает куки.
func (h *Handler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err == nil {
		_ = h.svc.Logout(c.Request.Context(), refreshToken)
	}

	h.clearTokenCookies(c)
	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func (h *Handler) setTokenCookies(c *gin.Context, tokens *TokenResponse) {
	c.SetCookie("access_token", tokens.AccessToken, 15*60, "/", h.cookieDomain, false, true)
	c.SetCookie("refresh_token", tokens.RefreshToken, 7*24*60*60, "/api/admin/", h.cookieDomain, false, true)
}

// PreviewAuth валидирует access_token для Caddy forward_auth. Возвращает 200 если токен валиден.
func (h *Handler) PreviewAuth(c *gin.Context) {
	c.Status(http.StatusOK)
}

// CreateAdmin создаёт нового администратора и возвращает сгенерированные login/password.
func (h *Handler) CreateAdmin(c *gin.Context) {
	var req CreateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ошибка валидации: " + err.Error()})
		return
	}

	resp, err := h.svc.CreateAdmin(c.Request.Context(), req.Role, req.Email)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// ListAdmins возвращает список всех администраторов.
func (h *Handler) ListAdmins(c *gin.Context) {
	// Получаем список из сервиса
	admins, err := h.svc.ListAdmins(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Ошибка получения списка администраторов: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"admins": admins})
}

// UpdateAdminRole меняет роль администратора по ID.
func (h *Handler) UpdateAdminRole(c *gin.Context) {
	// Парсим ID цели из URL-параметра
	targetID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Некорректный ID"})
		return
	}

	// Читаем тело запроса
	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ошибка валидации: " + err.Error()})
		return
	}

	callerID := c.GetInt("adminId")

	// Обновляем роль через сервис
	if err := h.svc.UpdateAdminRole(c.Request.Context(), callerID, targetID, req.Role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// DeleteAdmin удаляет администратора по ID.
func (h *Handler) DeleteAdmin(c *gin.Context) {
	targetID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Некорректный ID"})
		return
	}

	callerID := c.GetInt("adminId")

	if err := h.svc.DeleteAdmin(c.Request.Context(), callerID, targetID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

func (h *Handler) clearTokenCookies(c *gin.Context) {
	c.SetCookie("access_token", "", -1, "/", h.cookieDomain, false, true)
	c.SetCookie("refresh_token", "", -1, "/api/admin/", h.cookieDomain, false, true)
}

var validRequestStatuses = []string{"Новая", "На проверке", "Отклонена", "Опубликована"}

// GetRequests возвращает список заявок с опциональным фильтром по статусу и пагинацией.
func (h *Handler) GetRequests(c *gin.Context) {
	status := c.Query("status")

	// Проверяем статус если передан
	if status != "" && !slices.Contains(validRequestStatuses, status) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Недопустимый статус: " + status})
		return
	}

	// Парсим параметры пагинации
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Некорректный параметр page"})
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Параметр limit должен быть от 1 до 100"})
		return
	}

	q := c.Query("q")

	result, err := h.svc.ListRequests(c.Request.Context(), status, q, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Ошибка получения списка заявок: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"requests": result.Items, "total": result.Total, "page": page, "limit": limit})
}

// GetRequest возвращает полную карточку заявки по ID.
func (h *Handler) GetRequest(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Некорректный ID заявки"})
		return
	}

	req, err := h.svc.GetRequest(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Заявка не найдена"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"request": req})
}

// GetRequestHistory возвращает полную историю действий администраторов по всем заявкам.
func (h *Handler) GetRequestHistory(c *gin.Context) {
	q := c.Query("q")

	// Парсим параметры пагинации
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Некорректный параметр page"})
		return
	}
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Параметр limit должен быть от 1 до 100"})
		return
	}

	result, err := h.svc.GetRequestHistory(c.Request.Context(), q, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Ошибка получения истории: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"history": result.Items, "total": result.Total, "page": page, "limit": limit})
}
