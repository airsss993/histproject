package objects

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)


// Handler — HTTP-обработчики для объектов и типов событий.
type Handler struct {
	svc *Service
}

// NewHandler создаёт handler объектов.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// GetObjectDataReq — тело/параметры запроса получения данных объекта.
type GetObjectDataReq struct {
	ObjectId int `json:"objectId" binding:"required,gt=0"`
}

// GetObjectDataResp — ответ с данными одного объекта.
type GetObjectDataResp struct {
	Object SingleObjectInfo `json:"object"`
}

// GetObjectData возвращает данные одного объекта по ID.
func (h *Handler) GetObjectData(c *gin.Context) {
	objectIdParam := c.Param("id")
	objectId, err := strconv.Atoi(objectIdParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ошибка валидации: ID объекта должен быть числом"})
		return
	}

	obj, err := h.svc.GetObjectData(c.Request.Context(), objectId)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ошибка получения данных из БД: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"object": *obj})
}

// GetObjectsListReq — параметры фильтрации списка объектов.
type GetObjectsListReq struct {
	EventTypeIDs []int  `form:"eventTypeIds" binding:"omitempty,dive,gt=0"`
	DateFrom     string `form:"dateFrom" binding:"omitempty,datetime=2006-01-02"`
	DateTo       string `form:"dateTo" binding:"omitempty,datetime=2006-01-02"`
}

// GetObjectsListResp — ответ со списком объектов.
type GetObjectsListResp struct {
	Objects []ObjectInfo `json:"objects"`
}

// GetObjectsList возвращает список объектов с фильтрами.
func (h *Handler) GetObjectsList(c *gin.Context) {
	var req GetObjectsListReq
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ошибка парсинга запроса: " + err.Error()})
		return
	}

	list, err := h.svc.GetObjectsList(c.Request.Context(), req.EventTypeIDs, req.DateFrom, req.DateTo)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ошибка получения данных из БД: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetObjectsListResp{Objects: list})
}

// DeleteObject удаляет объект (метку) с карты по ID заявки.
func (h *Handler) DeleteObject(c *gin.Context) {
	// Парсим ID заявки из URL
	requestID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Некорректный ID заявки"})
		return
	}

	// Удаляем объект, привязанный к заявке
	if err := h.svc.DeleteObject(c.Request.Context(), requestID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

// UpdateCoordinatesReq — тело запроса на обновление координат.
type UpdateCoordinatesReq struct {
	Latitude  float64 `json:"latitude"  binding:"required"`
	Longitude float64 `json:"longitude" binding:"required"`
}

// SiteAuth используется Caddy forward_auth для проверки доступа к /sites/{id}/*.
// Возвращает 200 если заявка опубликована, 403 если нет.
func (h *Handler) SiteAuth(c *gin.Context) {
	// Caddy передаёт оригинальный URI: /sites/{slug}/index.html
	uri := c.GetHeader("X-Forwarded-Uri")
	uri = strings.TrimPrefix(uri, "/sites/")
	uri = strings.TrimPrefix(uri, "/")
	parts := strings.SplitN(uri, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		c.Status(http.StatusForbidden)
		return
	}
	siteSlug := parts[0]

	published, err := h.svc.IsPublished(c.Request.Context(), siteSlug)
	if err != nil || !published {
		c.Status(http.StatusForbidden)
		return
	}

	c.Status(http.StatusOK)
}

// UpdateCoordinates обновляет координаты метки по ID заявки.
func (h *Handler) UpdateCoordinates(c *gin.Context) {
	// Парсим ID заявки из URL
	requestID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Некорректный ID заявки"})
		return
	}

	// Парсим новые координаты из тела запроса
	var req UpdateCoordinatesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Ошибка валидации: " + err.Error()})
		return
	}

	// Обновляем координаты объекта, привязанного к заявке
	if err := h.svc.UpdateCoordinates(c.Request.Context(), requestID, req.Latitude, req.Longitude); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "success"})
}

