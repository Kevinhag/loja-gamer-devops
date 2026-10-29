package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"lojagamer/internal/store"
)

type errorBody struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorBody{Error: msg})
}

// writeStoreError traduz os erros do store em respostas HTTP.
func (s *Server) writeStoreError(w http.ResponseWriter, r *http.Request, err error, notFoundMsg string) {
	var stockErr *store.StockError
	switch {
	case errors.As(err, &stockErr):
		writeError(w, http.StatusConflict,
			fmt.Sprintf("Estoque insuficiente para %s (disponível: %d)", stockErr.Name, stockErr.Available))
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, notFoundMsg)
	case errors.Is(err, store.ErrEmptyCart):
		writeError(w, http.StatusBadRequest, "O carrinho está vazio")
	default:
		s.log.Error("erro no banco de dados", "erro", err.Error(), "path", r.URL.Path,
			"request_id", r.Context().Value(requestIDKey))
		writeError(w, http.StatusInternalServerError, "erro interno")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("JSON inválido: %w", err)
	}
	if dec.Decode(&struct{}{}) != io.EOF {
		return errors.New("JSON inválido: conteúdo extra após o objeto")
	}
	return nil
}

func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}
