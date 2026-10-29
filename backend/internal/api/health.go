package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"lojagamer/internal/images"
)

type checkResult struct {
	Status    string  `json:"status"`
	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
}

type healthBody struct {
	Status        string                 `json:"status"`
	Version       string                 `json:"version"`
	UptimeSeconds int64                  `json:"uptime_seconds"`
	Checks        map[string]checkResult `json:"checks"`
}

// handleLive só diz que o processo está de pé (usado pelo HEALTHCHECK do Docker).
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleHealth verifica as dependências (MySQL e storage). A pipeline usa este endpoint
// para validar o deploy, inclusive conferindo se a versão no ar é a do commit recém-publicado.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	body := healthBody{
		Status:        "ok",
		Version:       s.version,
		UptimeSeconds: int64(time.Since(s.started).Seconds()),
		Checks:        map[string]checkResult{},
	}
	run := func(name string, fn func(context.Context) error) {
		start := time.Now()
		res := checkResult{Status: "ok"}
		if err := fn(ctx); err != nil {
			res.Status = "fail"
			res.Error = err.Error()
			body.Status = "fail"
			s.metrics.healthFailures.WithLabelValues(name).Inc()
		}
		res.LatencyMS = float64(time.Since(start).Microseconds()) / 1000
		body.Checks[name] = res
	}

	run("database", s.store.Ping)
	if p, ok := s.images.(Pinger); ok {
		run("storage", p.Ping)
	}

	status := http.StatusOK
	if body.Status != "ok" {
		status = http.StatusServiceUnavailable
		s.log.Error("health check falhou", "checks", body.Checks)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, body)
}

func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if !images.ValidKey(key) {
		http.NotFound(w, r)
		return
	}
	rc, err := s.images.Get(r.Context(), key)
	if errors.Is(err, images.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Error("erro lendo imagem", "key", key, "erro", err.Error())
		http.Error(w, "erro ao carregar imagem", http.StatusBadGateway)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", images.ContentType(key))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = io.Copy(w, rc)
}

// staticHandler serve o build do Svelte. Rotas desconhecidas devolvem o index.html (SPA).
func (s *Server) staticHandler() http.Handler {
	index := filepath.Join(s.staticDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		s.log.Warn("frontend não encontrado; só a API será servida", "static_dir", s.staticDir)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeError(w, http.StatusNotFound, "rota não encontrada")
				return
			}
			http.Error(w, "frontend não compilado (rode npm run build em frontend/)", http.StatusNotFound)
		})
	}
	files := http.FileServer(http.Dir(s.staticDir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "rota não encontrada")
			return
		}
		clean := path.Clean("/" + r.URL.Path)
		if clean != "/" {
			if info, err := os.Stat(filepath.Join(s.staticDir, filepath.FromSlash(clean))); err == nil && !info.IsDir() {
				if strings.HasPrefix(clean, "/assets/") {
					// arquivos gerados pelo Vite têm hash no nome
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}
