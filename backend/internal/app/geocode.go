package app

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/airsss993/histproject/backend/internal/platform/httpserver"
)

const nominatimSearchURL = "https://nominatim.openstreetmap.org/search"

// Geocoder ищет место для заявки через Nominatim от имени сервера: IP и браузер посетителя
// за границу не уходят, уходит только текст запроса.
type Geocoder struct {
	client   *http.Client
	endpoint string
	logger   *slog.Logger
	mu       sync.Mutex
	last     time.Time
}

func NewGeocoder(logger *slog.Logger) *Geocoder {
	return &Geocoder{client: &http.Client{Timeout: 5 * time.Second}, endpoint: nominatimSearchURL, logger: logger}
}

type Place struct {
	DisplayName string `json:"display_name"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
}

func (geocoder *Geocoder) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/geocode", geocoder.search)
}

func (geocoder *Geocoder) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if n := utf8.RuneCountInString(query); n < 3 || n > 200 {
		_ = httpserver.WriteError(w, http.StatusBadRequest, "validation_failed", "Запрос от 3 до 200 символов", map[string]string{"q": "От 3 до 200 символов"})
		return
	}
	places, err := geocoder.lookup(r.Context(), query)
	if err != nil {
		geocoder.logger.WarnContext(r.Context(), "geocode failed", "request_id", httpserver.RequestIDFromContext(r.Context()), "error", err)
		_ = httpserver.WriteError(w, http.StatusBadGateway, "geocoder_unavailable", "Поиск места недоступен", nil)
		return
	}
	_ = httpserver.WriteJSON(w, http.StatusOK, map[string]any{"places": places})
}

// lookup держит не больше запроса в секунду на весь сервер - так требуют правила Nominatim.
// ponytail: общая очередь на мьютексе, при частом поиске нужен кеш ответов или свой геокодер.
func (geocoder *Geocoder) lookup(ctx context.Context, query string) ([]Place, error) {
	geocoder.mu.Lock()
	wait := time.Until(geocoder.last.Add(time.Second))
	if wait > 0 {
		select {
		case <-ctx.Done():
			geocoder.mu.Unlock()
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	geocoder.last = time.Now()
	geocoder.mu.Unlock()

	values := url.Values{"format": {"json"}, "limit": {"5"}, "accept-language": {"ru"}, "q": {query}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, geocoder.endpoint+"?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "russia-heroes.ru (info@russia-heroes.ru)")
	response, err := geocoder.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nominatim status %d", response.StatusCode)
	}
	places := []Place{}
	if err := json.NewDecoder(response.Body).Decode(&places); err != nil {
		return nil, fmt.Errorf("decode nominatim: %w", err)
	}
	return places, nil
}
