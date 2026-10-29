// Package api expõe a API HTTP da loja, o proxy de imagens, health checks, métricas e o frontend.
package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"lojagamer/internal/images"
	"lojagamer/internal/store"
)

const sessionCookie = "lg_session"

var sessionPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

// Pinger é implementado por dependências que o /healthz deve verificar (ex.: bucket S3).
type Pinger interface {
	Ping(ctx context.Context) error
}

type Options struct {
	Store        store.Store
	Images       images.Storage
	Logger       *slog.Logger
	AdminToken   string
	CookieSecure bool
	StaticDir    string
	Version      string
	Registry     *prometheus.Registry // nil cria um registry novo
}

type Server struct {
	store        store.Store
	images       images.Storage
	log          *slog.Logger
	adminToken   string
	cookieSecure bool
	staticDir    string
	version      string
	started      time.Time
	metrics      *metrics
}

func New(o Options) *Server {
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Registry == nil {
		o.Registry = prometheus.NewRegistry()
	}
	if o.Version == "" {
		o.Version = "dev"
	}
	return &Server{
		store:        o.Store,
		images:       o.Images,
		log:          o.Logger,
		adminToken:   o.AdminToken,
		cookieSecure: o.CookieSecure,
		staticDir:    o.StaticDir,
		version:      o.Version,
		started:      time.Now(),
		metrics:      newMetrics(o.Registry, o.Version),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /livez", s.handleLive)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.Handle("GET /metrics", s.metrics.handler())

	mux.HandleFunc("GET /api/products", s.handleListProducts)
	mux.HandleFunc("GET /api/products/{id}", s.handleGetProduct)
	mux.HandleFunc("POST /api/products", s.admin(s.handleCreateProduct))
	mux.HandleFunc("PUT /api/products/{id}", s.admin(s.handleUpdateProduct))
	mux.HandleFunc("DELETE /api/products/{id}", s.admin(s.handleDeleteProduct))
	mux.HandleFunc("GET /api/admin/session", s.admin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.HandleFunc("GET /api/cart", s.handleGetCart)
	mux.HandleFunc("POST /api/cart/items", s.handleAddToCart)
	mux.HandleFunc("PUT /api/cart/items/{productID}", s.handleSetCartQuantity)
	mux.HandleFunc("DELETE /api/cart/items/{productID}", s.handleRemoveFromCart)
	mux.HandleFunc("POST /api/checkout", s.handleCheckout)

	mux.HandleFunc("GET /images/{key...}", s.handleImage)
	mux.Handle("GET /", s.staticHandler())

	return s.withRequestID(s.withRecover(s.withObservability(mux)))
}

// admin exige o header "Authorization: Bearer <ADMIN_TOKEN>".
func (s *Server) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminToken == "" {
			writeError(w, http.StatusServiceUnavailable, "área admin desabilitada: defina ADMIN_TOKEN no servidor")
			return
		}
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "token de admin inválido")
			return
		}
		next(w, r)
	}
}

// session devolve o id do carrinho do visitante, criando o cookie na primeira visita.
func (s *Server) session(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil && sessionPattern.MatchString(c.Value) {
		return c.Value
	}
	id := randomHex(16)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   s.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	return id
}

type ctxKey int

const requestIDKey ctxKey = iota

func (s *Server) withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 64 {
			id = randomHex(8)
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func (s *Server) withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				s.log.Error("panic na requisição", "panic", v, "path", r.URL.Path,
					"request_id", r.Context().Value(requestIDKey), "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, "erro interno")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// withObservability registra um log estruturado e as métricas de cada requisição.
func (s *Server) withObservability(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start)

		route := r.Pattern // preenchido pelo ServeMux; vazio quando nenhuma rota casou
		if route == "" {
			route = "não encontrada"
		}
		s.metrics.observe(r.Method, route, rec.status, elapsed)

		if r.URL.Path == "/livez" || r.URL.Path == "/metrics" {
			return
		}
		level := slog.LevelInfo
		switch {
		case rec.status >= 500:
			level = slog.LevelError
		case rec.status >= 400:
			level = slog.LevelWarn
		}
		s.log.Log(r.Context(), level, "requisição",
			"method", r.Method,
			"path", r.URL.Path,
			"route", route,
			"status", rec.status,
			"duration_ms", float64(elapsed.Microseconds())/1000,
			"bytes", rec.bytes,
			"request_id", r.Context().Value(requestIDKey),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wrote = true
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
