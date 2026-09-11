package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"
)

type Server struct {
	server *http.Server
}

// New создаёт новый экземпляр HTTP-сервера.
func New(addr string, handler http.Handler) *Server {
	return &Server{server: &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}}
}

// Start запускает HTTP-сервер в отдельной горутине.
func (s *Server) Start() error {
	go func() {
		log.Println("[INFO] Сервер запущен на порту", s.server.Addr)
		// Запускаем сервер
		err := s.server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("[ERROR] Ошибка запуска сервера: ", err)
		}
	}()

	return nil
}

// Stop выполняет graceful shutdown сервера с учётом переданного контекста.
func (s *Server) Stop(ctx context.Context) error {
	if err := s.server.Shutdown(ctx); err != nil {
		log.Fatal("[ERROR] Ошибка при завершении сервера: ", err)
	}

	log.Println("[INFO] Сервер остановлен")

	return nil
}
