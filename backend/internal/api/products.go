package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"lojagamer/internal/images"
	"lojagamer/internal/store"
)

const maxImageBytes = 5 << 20 // 5 MB

var partTypePattern = regexp.MustCompile(`^[a-z][a-z_]{1,44}$`)

func withImageURL(p store.Product) store.Product {
	if p.ImageKey != "" {
		p.ImageURL = "/images/" + p.ImageKey
	}
	return p
}

func (s *Server) handleListProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f store.ProductFilter
	if v := q.Get("on_sale"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "on_sale deve ser true ou false")
			return
		}
		f.OnSale = &b
	}
	if v := q.Get("random"); v != "" {
		f.Random, _ = strconv.ParseBool(v)
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			writeError(w, http.StatusBadRequest, "limit deve estar entre 1 e 100")
			return
		}
		f.Limit = n
	}

	products, err := s.store.ListProducts(r.Context(), f)
	if err != nil {
		s.writeStoreError(w, r, err, "")
		return
	}
	for i := range products {
		products[i] = withImageURL(products[i])
	}
	writeJSON(w, http.StatusOK, products)
}

func (s *Server) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	p, err := s.store.GetProduct(r.Context(), id)
	if err != nil {
		s.writeStoreError(w, r, err, "Produto não encontrado")
		return
	}
	writeJSON(w, http.StatusOK, withImageURL(p))
}

func (s *Server) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	in, upload, ok := s.parseProductForm(w, r)
	if !ok {
		return
	}
	p, err := s.store.CreateProduct(r.Context(), in)
	if err != nil {
		s.writeStoreError(w, r, err, "")
		return
	}
	if upload != nil {
		if p, err = s.saveImage(r, p, upload); err != nil {
			// Sem a imagem o cadastro fica pela metade: desfaz para o admin tentar de novo.
			_ = s.store.DeleteProduct(r.Context(), p.ID)
			s.log.Error("falha ao salvar imagem", "erro", err.Error(), "request_id", r.Context().Value(requestIDKey))
			writeError(w, http.StatusBadGateway, "Não foi possível salvar a imagem do produto")
			return
		}
	}
	s.log.Info("produto criado", "product_id", p.ID, "name", p.Name)
	writeJSON(w, http.StatusCreated, withImageURL(p))
}

func (s *Server) handleUpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	in, upload, ok := s.parseProductForm(w, r)
	if !ok {
		return
	}
	p, err := s.store.UpdateProduct(r.Context(), id, in)
	if err != nil {
		s.writeStoreError(w, r, err, "Produto não encontrado")
		return
	}
	if upload != nil {
		oldKey := p.ImageKey
		if p, err = s.saveImage(r, p, upload); err != nil {
			s.log.Error("falha ao salvar imagem", "erro", err.Error(), "request_id", r.Context().Value(requestIDKey))
			writeError(w, http.StatusBadGateway, "Produto atualizado, mas não foi possível salvar a imagem")
			return
		}
		if oldKey != "" && oldKey != p.ImageKey {
			if err := s.images.Delete(r.Context(), oldKey); err != nil {
				s.log.Warn("não foi possível apagar a imagem antiga", "key", oldKey, "erro", err.Error())
			}
		}
	}
	s.log.Info("produto atualizado", "product_id", p.ID)
	writeJSON(w, http.StatusOK, withImageURL(p))
}

func (s *Server) handleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "id")
	if !ok {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	p, err := s.store.GetProduct(r.Context(), id)
	if err == nil {
		err = s.store.DeleteProduct(r.Context(), id)
	}
	if err != nil {
		s.writeStoreError(w, r, err, "Produto não encontrado")
		return
	}
	if p.ImageKey != "" {
		if err := s.images.Delete(r.Context(), p.ImageKey); err != nil {
			s.log.Warn("não foi possível apagar a imagem", "key", p.ImageKey, "erro", err.Error())
		}
	}
	s.log.Info("produto removido", "product_id", id, "name", p.Name)
	w.WriteHeader(http.StatusNoContent)
}

type imageUpload struct {
	data []byte
	ext  string
}

func (s *Server) saveImage(r *http.Request, p store.Product, up *imageUpload) (store.Product, error) {
	key := images.BuildKey(p.PartType, p.ID, p.Name, up.ext, time.Now().Unix())
	if err := s.images.Put(r.Context(), key, bytes.NewReader(up.data), int64(len(up.data)), images.ContentType(key)); err != nil {
		return p, err
	}
	if err := s.store.SetProductImage(r.Context(), p.ID, key); err != nil {
		return p, err
	}
	p.ImageKey = key
	return p, nil
}

// parseProductForm lê e valida o formulário multipart do admin. Em caso de erro já escreve a resposta.
func (s *Server) parseProductForm(w http.ResponseWriter, r *http.Request) (store.ProductInput, *imageUpload, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+(1<<20))
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "A imagem deve ter no máximo 5 MB")
		} else {
			writeError(w, http.StatusBadRequest, "Envie os dados como multipart/form-data")
		}
		return store.ProductInput{}, nil, false
	}
	defer r.MultipartForm.RemoveAll()

	in, fields := parseProductFields(r.FormValue)
	upload, err := readImage(r)
	if err != nil {
		fields["image"] = err.Error()
	}
	if len(fields) > 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "Dados do produto inválidos", Fields: fields})
		return in, nil, false
	}
	return in, upload, true
}

func parseProductFields(get func(string) string) (store.ProductInput, map[string]string) {
	fields := map[string]string{}
	text := func(name string, max int, required bool) string {
		v := strings.TrimSpace(get(name))
		switch {
		case required && v == "":
			fields[name] = "obrigatório"
		case len([]rune(v)) > max:
			fields[name] = fmt.Sprintf("máximo de %d caracteres", max)
		}
		return v
	}

	in := store.ProductInput{
		Name:         text("name", 255, true),
		Brand:        text("brand", 255, false),
		Description:  text("description", 1000, false),
		Manufacturer: text("manufacturer", 100, false),
		ProductLine:  text("product_line", 100, false),
		Model:        text("model", 100, false),
		IsNew:        parseFlag(get("is_new")),
		OnSale:       parseFlag(get("on_sale")),
	}

	in.PartType = strings.ReplaceAll(strings.ToLower(strings.TrimSpace(get("part_type"))), " ", "_")
	if !partTypePattern.MatchString(in.PartType) {
		fields["part_type"] = "use letras minúsculas e _ (ex.: graphics_card)"
	}

	price, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(get("price")), ",", ".", 1), 64)
	if err != nil || price < 0 || price > 99_999_999 {
		fields["price"] = "preço inválido"
	}
	in.Price = price

	stock, err := strconv.Atoi(strings.TrimSpace(get("stock")))
	if err != nil || stock < 0 {
		fields["stock"] = "estoque deve ser um inteiro maior ou igual a 0"
	}
	in.Stock = stock

	rating := 0
	if v := strings.TrimSpace(get("rating")); v != "" {
		rating, err = strconv.Atoi(v)
		if err != nil || rating < 0 || rating > 5 {
			fields["rating"] = "avaliação deve ser de 0 a 5"
		}
	}
	in.Rating = rating

	return in, fields
}

func parseFlag(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "on", "sim", "yes":
		return true
	}
	return false
}

// readImage lê o campo "image" (opcional) e confere o tipo real do arquivo, não só a extensão.
func readImage(r *http.Request) (*imageUpload, error) {
	file, _, err := r.FormFile("image")
	if errors.Is(err, http.ErrMissingFile) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("não foi possível ler a imagem")
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxImageBytes+1))
	if err != nil {
		return nil, errors.New("não foi possível ler a imagem")
	}
	if len(data) == 0 {
		return nil, nil
	}
	if len(data) > maxImageBytes {
		return nil, errors.New("a imagem deve ter no máximo 5 MB")
	}
	switch http.DetectContentType(data) {
	case "image/png":
		return &imageUpload{data: data, ext: "png"}, nil
	case "image/jpeg":
		return &imageUpload{data: data, ext: "jpg"}, nil
	case "image/webp":
		return &imageUpload{data: data, ext: "webp"}, nil
	}
	return nil, errors.New("formato não suportado (use PNG, JPG ou WebP)")
}
