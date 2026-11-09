package httptransport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
)

type Server struct {
	Store *postgres.Store
	Log   *slog.Logger
}
type requestIDKey struct{}

func New(store *postgres.Store, log *slog.Logger) http.Handler {
	s := &Server{Store: store, Log: log}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { JSON(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := store.Pool.Ping(ctx); err != nil {
			JSON(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		JSON(w, 200, map[string]string{"status": "ready"})
	})
	return s.middleware(mux)
}
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if status != 204 {
		json.NewEncoder(w).Encode(v)
	}
}
func (s *Server) Error(w http.ResponseWriter, r *http.Request, err error) {
	err = postgres.MapError(err)
	e := &domain.Error{Code: "INTERNAL", Status: 500, Message: "Внутренняя ошибка."}
	if !errors.As(err, &e) {
		s.Log.ErrorContext(r.Context(), "request failed", "request_id", r.Context().Value(requestIDKey{}), "error_type", "internal")
	}
	JSON(w, e.Status, map[string]any{"error": map[string]any{"code": e.Code, "message": e.Message, "details": e.Details, "requestId": r.Context().Value(requestIDKey{})}})
}
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 16)
		rand.Read(b)
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id))
		defer func() {
			if recover() != nil {
				s.Log.Error("request panic", "request_id", id)
				s.Error(w, r, errors.New("panic"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
