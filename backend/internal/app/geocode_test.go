package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeocoderProxiesQueryWithoutClientHeaders(t *testing.T) {
	var got *http.Request
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		_, _ = io.WriteString(w, `[{"display_name":"Екатеринбург","lat":"56.8","lon":"60.6","osm_id":1}]`)
	}))
	defer upstream.Close()
	geocoder := NewGeocoder(slog.New(slog.NewTextHandler(io.Discard, nil)))
	geocoder.endpoint = upstream.URL
	mux := http.NewServeMux()
	geocoder.Register(mux)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/geocode?q=%D0%95%D0%BA%D0%B0%D1%82", nil)
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	request.Header.Set("Cookie", "histproject_user=secret")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"places":[{"display_name":"Екатеринбург","lat":"56.8","lon":"60.6"}]`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	if got.URL.Query().Get("q") != "Екат" || got.Header.Get("X-Forwarded-For") != "" || got.Header.Get("Cookie") != "" {
		t.Fatalf("upstream request = %v %v", got.URL, got.Header)
	}

	short := httptest.NewRecorder()
	mux.ServeHTTP(short, httptest.NewRequest(http.MethodGet, "/api/v1/geocode?q=ab", nil))
	if short.Code != http.StatusBadRequest {
		t.Fatalf("short query status = %d", short.Code)
	}
}
