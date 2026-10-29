package api

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lojagamer/internal/images"
	"lojagamer/internal/store"
)

const testToken = "segredo-de-teste"

type env struct {
	t       *testing.T
	srv     *httptest.Server
	client  *http.Client
	store   *fakeStore
	storage *images.Local
}

func seedProducts() []store.Product {
	return []store.Product{
		{ID: 1, Name: "RTX 3060", Brand: "NVIDIA", Price: 2099.99, Stock: 5, Rating: 5, OnSale: true, PartType: "graphics_card", ImageKey: "graphics_card/rtx-3060.webp"},
		{ID: 2, Name: "Ryzen 7", Brand: "AMD", Price: 1999.99, Stock: 2, Rating: 4, PartType: "cpu"},
		{ID: 3, Name: "<script>alert(1)</script>", Brand: "X", Price: 10, Stock: 1, PartType: "ram"},
	}
}

func newEnv(t *testing.T, mutate ...func(*Options)) *env {
	t.Helper()
	fs := newFakeStore(seedProducts()...)
	storage, err := images.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	opts := Options{
		Store:      fs,
		Images:     storage,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		AdminToken: testToken,
		StaticDir:  t.TempDir(),
		Version:    "test-version",
	}
	for _, m := range mutate {
		m(&opts)
	}
	srv := httptest.NewServer(New(opts).Handler())
	t.Cleanup(srv.Close)
	return &env{t: t, srv: srv, client: newClient(), store: fs, storage: storage}
}

func newClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func (e *env) do(c *http.Client, method, path string, body io.Reader, headers map[string]string) *http.Response {
	e.t.Helper()
	req, err := http.NewRequest(method, e.srv.URL+path, body)
	if err != nil {
		e.t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if c == nil {
		c = e.client
	}
	resp, err := c.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func (e *env) json(method, path string, body any) *http.Response {
	e.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	return e.do(nil, method, path, r, map[string]string{"Content-Type": "application/json"})
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decodificando resposta: %v", err)
	}
	return v
}

func expectStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: status %d, esperado %d. corpo: %s", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, want, body)
	}
}

func pngBytes(t *testing.T) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// productForm monta o multipart enviado pelo admin.
func productForm(t *testing.T, fields map[string]string, file []byte) (io.Reader, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		w.WriteField(k, v)
	}
	if file != nil {
		fw, _ := w.CreateFormFile("image", "foto.png")
		fw.Write(file)
	}
	w.Close()
	return &buf, w.FormDataContentType()
}

func validFields() map[string]string {
	return map[string]string{
		"name": "GeForce RTX 5090", "brand": "NVIDIA", "price": "15999,90", "stock": "3", "rating": "5",
		"is_new": "true", "on_sale": "false", "part_type": "graphics_card", "description": "32GB GDDR7",
	}
}

func (e *env) admin(method, path string, fields map[string]string, file []byte) *http.Response {
	body, ctype := productForm(e.t, fields, file)
	return e.do(nil, method, path, body, map[string]string{"Content-Type": ctype, "Authorization": "Bearer " + testToken})
}

// ---------------------------------------------------------------------------------------------

func TestLivez(t *testing.T) {
	e := newEnv(t)
	expectStatus(t, e.do(nil, "GET", "/livez", nil, nil), http.StatusOK)
}

func TestHealthOK(t *testing.T) {
	e := newEnv(t)
	resp := e.do(nil, "GET", "/healthz", nil, nil)
	expectStatus(t, resp, http.StatusOK)
	h := decode[healthBody](t, resp)
	if h.Status != "ok" || h.Version != "test-version" || h.Checks["database"].Status != "ok" {
		t.Fatalf("health inesperado: %+v", h)
	}
}

func TestHealthReportsDatabaseFailure(t *testing.T) {
	e := newEnv(t)
	e.store.pingErr = errDB
	resp := e.do(nil, "GET", "/healthz", nil, nil)
	expectStatus(t, resp, http.StatusServiceUnavailable)
	h := decode[healthBody](t, resp)
	if h.Status != "fail" || h.Checks["database"].Error != errDB.Error() {
		t.Fatalf("health deveria acusar o banco: %+v", h)
	}

	metrics, _ := io.ReadAll(e.do(nil, "GET", "/metrics", nil, nil).Body)
	if !strings.Contains(string(metrics), `lojagamer_health_check_failures_total{check="database"} 1`) {
		t.Fatal("métrica de falha do health check não foi incrementada")
	}
}

type pingStorage struct {
	*images.Local
	err error
}

func (p pingStorage) Ping(context.Context) error { return p.err }

func TestHealthChecksStorageWhenSupported(t *testing.T) {
	local, _ := images.NewLocal(t.TempDir())
	e := newEnv(t, func(o *Options) { o.Images = pingStorage{Local: local, err: io.ErrUnexpectedEOF} })
	resp := e.do(nil, "GET", "/healthz", nil, nil)
	expectStatus(t, resp, http.StatusServiceUnavailable)
	if h := decode[healthBody](t, resp); h.Checks["storage"].Status != "fail" {
		t.Fatalf("storage deveria falhar: %+v", h)
	}
}

func TestListProducts(t *testing.T) {
	e := newEnv(t)

	all := decode[[]store.Product](t, e.do(nil, "GET", "/api/products", nil, nil))
	if len(all) != 3 {
		t.Fatalf("esperava 3 produtos, veio %d", len(all))
	}
	if all[0].ImageURL != "/images/graphics_card/rtx-3060.webp" || all[1].ImageURL != "" {
		t.Fatalf("image_url incorreta: %q / %q", all[0].ImageURL, all[1].ImageURL)
	}

	promo := decode[[]store.Product](t, e.do(nil, "GET", "/api/products?on_sale=true&random=true&limit=5", nil, nil))
	if len(promo) != 1 || promo[0].ID != 1 {
		t.Fatalf("filtro on_sale falhou: %+v", promo)
	}

	expectStatus(t, e.do(nil, "GET", "/api/products?limit=0", nil, nil), http.StatusBadRequest)
	expectStatus(t, e.do(nil, "GET", "/api/products?limit=500", nil, nil), http.StatusBadRequest)
	expectStatus(t, e.do(nil, "GET", "/api/products?on_sale=talvez", nil, nil), http.StatusBadRequest)
}

func TestProductJSONDoesNotExposeImageKey(t *testing.T) {
	e := newEnv(t)
	body, _ := io.ReadAll(e.do(nil, "GET", "/api/products/1", nil, nil).Body)
	if strings.Contains(string(body), "image_key") {
		t.Fatalf("image_key não deveria aparecer no JSON: %s", body)
	}
}

func TestGetProduct(t *testing.T) {
	e := newEnv(t)
	resp := e.do(nil, "GET", "/api/products/2", nil, nil)
	expectStatus(t, resp, http.StatusOK)
	if p := decode[store.Product](t, resp); p.Name != "Ryzen 7" {
		t.Fatalf("produto errado: %+v", p)
	}
	expectStatus(t, e.do(nil, "GET", "/api/products/999", nil, nil), http.StatusNotFound)
	expectStatus(t, e.do(nil, "GET", "/api/products/abc", nil, nil), http.StatusBadRequest)
}

func TestAdminRoutesRequireToken(t *testing.T) {
	e := newEnv(t)
	body, ctype := productForm(t, validFields(), nil)
	expectStatus(t, e.do(nil, "POST", "/api/products", body, map[string]string{"Content-Type": ctype}), http.StatusUnauthorized)
	expectStatus(t, e.do(nil, "DELETE", "/api/products/1", nil, map[string]string{"Authorization": "Bearer errado"}), http.StatusUnauthorized)
	expectStatus(t, e.do(nil, "GET", "/api/admin/session", nil, map[string]string{"Authorization": "Bearer " + testToken}), http.StatusNoContent)

	if _, err := e.store.GetProduct(context.Background(), 1); err != nil {
		t.Fatal("produto não deveria ter sido removido sem token")
	}
}

func TestAdminDisabledWithoutConfiguredToken(t *testing.T) {
	e := newEnv(t, func(o *Options) { o.AdminToken = "" })
	resp := e.do(nil, "GET", "/api/admin/session", nil, map[string]string{"Authorization": "Bearer "})
	expectStatus(t, resp, http.StatusServiceUnavailable)
}

func TestCreateProductWithImage(t *testing.T) {
	e := newEnv(t)
	img := pngBytes(t)
	resp := e.admin("POST", "/api/products", validFields(), img)
	expectStatus(t, resp, http.StatusCreated)
	p := decode[store.Product](t, resp)

	if p.Name != "GeForce RTX 5090" || p.Price != 15999.90 || p.Stock != 3 || !p.IsNew || p.OnSale {
		t.Fatalf("produto criado incorretamente: %+v", p)
	}
	if !strings.HasPrefix(p.ImageURL, "/images/graphics_card/4-geforce-rtx-5090-") || !strings.HasSuffix(p.ImageURL, ".png") {
		t.Fatalf("image_url inesperada: %q", p.ImageURL)
	}

	imgResp := e.do(nil, "GET", p.ImageURL, nil, nil)
	expectStatus(t, imgResp, http.StatusOK)
	got, _ := io.ReadAll(imgResp.Body)
	if !bytes.Equal(got, img) || imgResp.Header.Get("Content-Type") != "image/png" {
		t.Fatal("a imagem servida não é a enviada")
	}
}

func TestCreateProductValidation(t *testing.T) {
	e := newEnv(t)
	fields := validFields()
	fields["name"] = "  "
	fields["price"] = "abc"
	fields["rating"] = "9"
	fields["stock"] = "-1"
	fields["part_type"] = "../../etc"

	resp := e.admin("POST", "/api/products", fields, nil)
	expectStatus(t, resp, http.StatusBadRequest)
	body := decode[errorBody](t, resp)
	for _, f := range []string{"name", "price", "rating", "stock", "part_type"} {
		if body.Fields[f] == "" {
			t.Errorf("esperava erro no campo %q: %+v", f, body.Fields)
		}
	}
	if len(e.store.products) != 3 {
		t.Fatal("nenhum produto deveria ter sido criado")
	}
}

func TestCreateProductRejectsFakeImage(t *testing.T) {
	e := newEnv(t)
	resp := e.admin("POST", "/api/products", validFields(), []byte("<?php system($_GET['c']); ?>"))
	expectStatus(t, resp, http.StatusBadRequest)
	if body := decode[errorBody](t, resp); body.Fields["image"] == "" {
		t.Fatalf("esperava erro no campo image: %+v", body)
	}
}

func TestCreateProductRejectsHugeUpload(t *testing.T) {
	e := newEnv(t)
	resp := e.admin("POST", "/api/products", validFields(), bytes.Repeat([]byte{0}, maxImageBytes+(2<<20)))
	expectStatus(t, resp, http.StatusRequestEntityTooLarge)
}

func TestUpdateProductReplacesImage(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	oldKey := "graphics_card/rtx-3060.webp"
	e.storage.Put(ctx, oldKey, bytes.NewReader([]byte("antiga")), 6, "image/webp")

	fields := validFields()
	fields["name"] = "RTX 3060 OC"
	resp := e.admin("PUT", "/api/products/1", fields, pngBytes(t))
	expectStatus(t, resp, http.StatusOK)
	p := decode[store.Product](t, resp)

	if p.Name != "RTX 3060 OC" || p.ImageURL == "/images/"+oldKey {
		t.Fatalf("update incorreto: %+v", p)
	}
	if ok, _ := e.storage.Exists(ctx, oldKey); ok {
		t.Fatal("a imagem antiga deveria ter sido apagada")
	}
}

func TestUpdateProductKeepsImageWhenNoneSent(t *testing.T) {
	e := newEnv(t)
	resp := e.admin("PUT", "/api/products/1", validFields(), nil)
	expectStatus(t, resp, http.StatusOK)
	if p := decode[store.Product](t, resp); p.ImageURL != "/images/graphics_card/rtx-3060.webp" {
		t.Fatalf("a imagem deveria ser mantida: %q", p.ImageURL)
	}
	expectStatus(t, e.admin("PUT", "/api/products/999", validFields(), nil), http.StatusNotFound)
}

func TestDeleteProduct(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.storage.Put(ctx, "graphics_card/rtx-3060.webp", bytes.NewReader([]byte("x")), 1, "image/webp")
	auth := map[string]string{"Authorization": "Bearer " + testToken}

	expectStatus(t, e.do(nil, "DELETE", "/api/products/1", nil, auth), http.StatusNoContent)
	expectStatus(t, e.do(nil, "DELETE", "/api/products/1", nil, auth), http.StatusNotFound)
	if ok, _ := e.storage.Exists(ctx, "graphics_card/rtx-3060.webp"); ok {
		t.Fatal("a imagem do produto removido deveria ter sido apagada")
	}
}

func TestCartFlow(t *testing.T) {
	e := newEnv(t)

	resp := e.json("POST", "/api/cart/items", map[string]any{"product_id": 1, "quantity": 2})
	expectStatus(t, resp, http.StatusOK)
	if len(resp.Cookies()) == 0 || resp.Cookies()[0].Name != sessionCookie || !resp.Cookies()[0].HttpOnly {
		t.Fatal("o cookie de sessão HttpOnly deveria ter sido criado")
	}
	cart := decode[store.Cart](t, resp)
	if cart.TotalItems != 2 || cart.TotalPrice != 4199.98 || cart.Items[0].Product.ImageURL == "" {
		t.Fatalf("carrinho inesperado: %+v", cart)
	}

	resp = e.json("POST", "/api/cart/items", map[string]any{"product_id": 1, "quantity": 4})
	expectStatus(t, resp, http.StatusConflict)
	if msg := decode[errorBody](t, resp).Error; !strings.Contains(msg, "disponível: 5") {
		t.Fatalf("mensagem de estoque inesperada: %q", msg)
	}

	resp = e.json("PUT", "/api/cart/items/1", map[string]any{"quantity": 1})
	expectStatus(t, resp, http.StatusOK)
	if c := decode[store.Cart](t, resp); c.TotalItems != 1 {
		t.Fatalf("quantidade não foi atualizada: %+v", c)
	}

	// Outro visitante (outro cookie) tem um carrinho separado.
	other := decode[store.Cart](t, e.do(newClient(), "GET", "/api/cart", nil, nil))
	if len(other.Items) != 0 {
		t.Fatalf("carrinho vazou entre sessões: %+v", other)
	}

	resp = e.do(nil, "DELETE", "/api/cart/items/1", nil, nil)
	expectStatus(t, resp, http.StatusOK)
	if c := decode[store.Cart](t, resp); len(c.Items) != 0 {
		t.Fatalf("item não foi removido: %+v", c)
	}
	expectStatus(t, e.do(nil, "DELETE", "/api/cart/items/1", nil, nil), http.StatusNotFound)
}

func TestCartValidation(t *testing.T) {
	e := newEnv(t)
	expectStatus(t, e.json("POST", "/api/cart/items", map[string]any{"product_id": 999}), http.StatusNotFound)
	expectStatus(t, e.json("POST", "/api/cart/items", map[string]any{"product_id": 1, "quantity": 100}), http.StatusBadRequest)
	expectStatus(t, e.json("POST", "/api/cart/items", map[string]any{"product_id": 1, "extra": true}), http.StatusBadRequest)
	expectStatus(t, e.do(nil, "POST", "/api/cart/items", strings.NewReader("{"), nil), http.StatusBadRequest)
	expectStatus(t, e.json("PUT", "/api/cart/items/1", map[string]any{"quantity": -1}), http.StatusBadRequest)
}

func TestCheckout(t *testing.T) {
	e := newEnv(t)
	resp := e.json("POST", "/api/checkout", map[string]any{"payment_method": "pix"})
	expectStatus(t, resp, http.StatusBadRequest)
	if msg := decode[errorBody](t, resp).Error; msg != "O carrinho está vazio" {
		t.Fatalf("mensagem inesperada: %q", msg)
	}

	expectStatus(t, e.json("POST", "/api/cart/items", map[string]any{"product_id": 2, "quantity": 2}), http.StatusOK)
	expectStatus(t, e.json("POST", "/api/checkout", map[string]any{"payment_method": "boleto"}), http.StatusBadRequest)

	resp = e.json("POST", "/api/checkout", map[string]any{"payment_method": "credito"})
	expectStatus(t, resp, http.StatusCreated)
	order := decode[store.Order](t, resp)
	if order.ID != 1 || order.TotalItems != 2 || order.PaymentMethod != "credito" {
		t.Fatalf("pedido inesperado: %+v", order)
	}
	if p, _ := e.store.GetProduct(context.Background(), 2); p.Stock != 0 {
		t.Fatalf("estoque deveria ter zerado, está %d", p.Stock)
	}

	metrics, _ := io.ReadAll(e.do(nil, "GET", "/metrics", nil, nil).Body)
	if !strings.Contains(string(metrics), `lojagamer_orders_total{payment_method="credito"} 1`) {
		t.Fatal("métrica de pedidos não foi incrementada")
	}
}

func TestDatabaseErrorsDoNotLeakDetails(t *testing.T) {
	e := newEnv(t)
	e.store.failWith = errDB
	resp := e.do(nil, "GET", "/api/products", nil, nil)
	expectStatus(t, resp, http.StatusInternalServerError)
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), errDB.Error()) {
		t.Fatalf("detalhe interno vazou na resposta: %s", body)
	}
}

func TestImageRoute(t *testing.T) {
	e := newEnv(t)
	expectStatus(t, e.do(nil, "GET", "/images/cpu/nao-existe.webp", nil, nil), http.StatusNotFound)
	expectStatus(t, e.do(nil, "GET", "/images/..%2f..%2fetc/passwd", nil, nil), http.StatusNotFound)
	expectStatus(t, e.do(nil, "GET", "/images/cpu/../../segredo.png", nil, nil), http.StatusNotFound)
}

func TestStaticSPA(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<div id=app></div>"), 0o644)
	os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
	os.WriteFile(filepath.Join(dir, "assets", "app-abc123.js"), []byte("console.log(1)"), 0o644)
	e := newEnv(t, func(o *Options) { o.StaticDir = dir })

	for _, path := range []string{"/", "/carrinho", "/admin/produtos"} {
		resp := e.do(nil, "GET", path, nil, nil)
		expectStatus(t, resp, http.StatusOK)
		if body, _ := io.ReadAll(resp.Body); string(body) != "<div id=app></div>" {
			t.Fatalf("%s deveria devolver o index.html, veio %q", path, body)
		}
	}

	resp := e.do(nil, "GET", "/assets/app-abc123.js", nil, nil)
	expectStatus(t, resp, http.StatusOK)
	if !strings.Contains(resp.Header.Get("Cache-Control"), "immutable") {
		t.Fatal("assets com hash deveriam ter cache longo")
	}

	resp = e.do(nil, "GET", "/api/nao-existe", nil, nil)
	expectStatus(t, resp, http.StatusNotFound)
	if resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatal("rotas /api desconhecidas devem responder JSON, não o index.html")
	}
}

func TestMetricsAndRequestID(t *testing.T) {
	e := newEnv(t)
	resp := e.do(nil, "GET", "/api/products", nil, map[string]string{"X-Request-ID": "abc-123"})
	if resp.Header.Get("X-Request-ID") != "abc-123" {
		t.Fatal("X-Request-ID deveria ser propagado")
	}
	if e.do(nil, "GET", "/livez", nil, nil).Header.Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID deveria ser gerado quando ausente")
	}

	metrics, _ := io.ReadAll(e.do(nil, "GET", "/metrics", nil, nil).Body)
	for _, want := range []string{
		`http_requests_total{method="GET",route="GET /api/products",status="200"} 1`,
		`lojagamer_build_info{version="test-version"} 1`,
		`http_request_duration_seconds_bucket`,
	} {
		if !strings.Contains(string(metrics), want) {
			t.Errorf("métrica ausente: %s", want)
		}
	}
}

func TestParseFlag(t *testing.T) {
	for _, v := range []string{"1", "true", "on", "SIM", " yes "} {
		if !parseFlag(v) {
			t.Errorf("%q deveria ser verdadeiro", v)
		}
	}
	for _, v := range []string{"", "0", "false", "não"} {
		if parseFlag(v) {
			t.Errorf("%q deveria ser falso", v)
		}
	}
}
