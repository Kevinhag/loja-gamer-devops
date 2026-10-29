package store_test

// Testes de integração contra um MySQL real. Rodam quando TEST_MYSQL_DSN está definido
// (a pipeline sobe um container mysql:8.4 para isso); caso contrário são ignorados.
//
//   TEST_MYSQL_DSN="root:test@tcp(127.0.0.1:3306)/lojagamer_test?parseTime=true&multiStatements=true" go test ./...

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"lojagamer/internal/database"
	"lojagamer/internal/store"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		os.Exit(m.Run()) // os testes chamam requireDB e são ignorados
	}
	ctx := context.Background()
	db, err := database.Open(ctx, dsn, 60*time.Second)
	if err != nil {
		panic(err)
	}
	if _, err := db.Exec("DROP TABLE IF EXISTS order_items, orders, cart_items, products, schema_migrations"); err != nil {
		panic(err)
	}
	if err := database.Migrate(ctx, db); err != nil {
		panic(err)
	}
	testDB = db
	code := m.Run()
	db.Close()
	os.Exit(code)
}

func requireDB(t *testing.T) *store.MySQL {
	t.Helper()
	if testDB == nil {
		t.Skip("TEST_MYSQL_DSN não definido; pulando teste de integração")
	}
	return store.NewMySQL(testDB)
}

func newSession(t *testing.T) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func createProduct(t *testing.T, s *store.MySQL, name string, price float64, stock int) store.Product {
	t.Helper()
	p, err := s.CreateProduct(context.Background(), store.ProductInput{
		Name: name, Brand: "Teste", Price: price, Stock: stock, Rating: 3, PartType: "cpu",
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	return p
}

func TestMigrationsAreIdempotentAndSeedProducts(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	if err := database.Migrate(ctx, testDB); err != nil {
		t.Fatalf("segunda execução das migrations falhou: %v", err)
	}
	var seeded int
	if err := testDB.QueryRow("SELECT COUNT(*) FROM products WHERE image_key <> ''").Scan(&seeded); err != nil {
		t.Fatal(err)
	}
	if seeded != 50 {
		t.Fatalf("esperava 50 produtos do seed, veio %d", seeded)
	}

	onSale := true
	promo, err := s.ListProducts(ctx, store.ProductFilter{OnSale: &onSale, Random: true, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(promo) != 5 {
		t.Fatalf("esperava 5 produtos em promoção, veio %d", len(promo))
	}
	for _, p := range promo {
		if !p.OnSale {
			t.Fatalf("produto %q não está em promoção", p.Name)
		}
	}
}

func TestProductCRUD(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()

	p := createProduct(t, s, "Produto CRUD", 199.9, 4)
	if p.ID == 0 || p.Price != 199.9 || p.CreatedAt.IsZero() {
		t.Fatalf("produto criado incorretamente: %+v", p)
	}

	in := store.ProductInput{Name: "Produto CRUD v2", Brand: "Outra", Price: 250, Stock: 9, Rating: 5,
		IsNew: true, OnSale: true, PartType: "ram", Description: "desc"}
	upd, err := s.UpdateProduct(ctx, p.ID, in)
	if err != nil {
		t.Fatal(err)
	}
	if upd.Name != in.Name || upd.Stock != 9 || !upd.IsNew || !upd.OnSale || upd.PartType != "ram" {
		t.Fatalf("update não persistiu: %+v", upd)
	}
	// Salvar os mesmos dados de novo não pode virar "não encontrado".
	if _, err := s.UpdateProduct(ctx, p.ID, in); err != nil {
		t.Fatalf("update idempotente falhou: %v", err)
	}

	if err := s.SetProductImage(ctx, p.ID, "ram/x.webp"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetProduct(ctx, p.ID)
	if got.ImageKey != "ram/x.webp" {
		t.Fatalf("image_key = %q", got.ImageKey)
	}

	if err := s.DeleteProduct(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetProduct(ctx, p.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("esperava ErrNotFound depois do delete, veio %v", err)
	}
	if err := s.DeleteProduct(ctx, p.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("delete repetido deveria dar ErrNotFound, veio %v", err)
	}
	if _, err := s.UpdateProduct(ctx, p.ID, in); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("update de produto inexistente deveria dar ErrNotFound, veio %v", err)
	}
}

func TestCartRespectsStockAndSessions(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	p := createProduct(t, s, "Produto Carrinho", 100.10, 3)
	sess, other := newSession(t), newSession(t)

	if err := s.AddToCart(ctx, sess, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.AddToCart(ctx, sess, p.ID, 1); err != nil {
		t.Fatal(err)
	}
	cart, err := s.GetCart(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart.Items) != 1 || cart.Items[0].Quantity != 2 || cart.TotalItems != 2 || cart.TotalPrice != 200.20 {
		t.Fatalf("carrinho inesperado: %+v", cart)
	}

	var stockErr *store.StockError
	if err := s.AddToCart(ctx, sess, p.ID, 2); !errors.As(err, &stockErr) || stockErr.Available != 3 {
		t.Fatalf("esperava StockError com 3 disponíveis, veio %v", err)
	}
	if err := s.SetCartQuantity(ctx, sess, p.ID, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCartQuantity(ctx, sess, p.ID, 4); !errors.Is(err, store.ErrOutOfStock) {
		t.Fatalf("esperava ErrOutOfStock, veio %v", err)
	}

	// Carrinhos de sessões diferentes são independentes.
	if c, _ := s.GetCart(ctx, other); len(c.Items) != 0 {
		t.Fatalf("outra sessão enxergou o carrinho: %+v", c)
	}
	if err := s.AddToCart(ctx, sess, 999999, 1); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("produto inexistente deveria dar ErrNotFound, veio %v", err)
	}

	if err := s.SetCartQuantity(ctx, sess, p.ID, 0); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.GetCart(ctx, sess); len(c.Items) != 0 {
		t.Fatalf("quantidade 0 deveria remover o item: %+v", c)
	}
	if err := s.RemoveFromCart(ctx, sess, p.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("remover item ausente deveria dar ErrNotFound, veio %v", err)
	}
}

func TestDeletingProductRemovesItFromCarts(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	p := createProduct(t, s, "Produto Removido", 10, 5)
	sess := newSession(t)
	if err := s.AddToCart(ctx, sess, p.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteProduct(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.GetCart(ctx, sess); len(c.Items) != 0 {
		t.Fatalf("item de produto removido continuou no carrinho: %+v", c)
	}
}

func TestCheckout(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	a := createProduct(t, s, "Checkout A", 50.25, 5)
	b := createProduct(t, s, "Checkout B", 10, 2)
	sess := newSession(t)

	if _, err := s.Checkout(ctx, sess, "pix"); !errors.Is(err, store.ErrEmptyCart) {
		t.Fatalf("checkout vazio deveria dar ErrEmptyCart, veio %v", err)
	}

	must(t, s.AddToCart(ctx, sess, a.ID, 2))
	must(t, s.AddToCart(ctx, sess, b.ID, 1))

	order, err := s.Checkout(ctx, sess, "pix")
	if err != nil {
		t.Fatal(err)
	}
	if order.ID == 0 || order.Total != 110.50 || order.TotalItems != 3 || order.PaymentMethod != "pix" {
		t.Fatalf("pedido inesperado: %+v", order)
	}

	if got, _ := s.GetProduct(ctx, a.ID); got.Stock != 3 {
		t.Fatalf("estoque de A deveria ser 3, é %d", got.Stock)
	}
	if got, _ := s.GetProduct(ctx, b.ID); got.Stock != 1 {
		t.Fatalf("estoque de B deveria ser 1, é %d", got.Stock)
	}
	if c, _ := s.GetCart(ctx, sess); len(c.Items) != 0 {
		t.Fatalf("carrinho deveria estar vazio após o checkout: %+v", c)
	}
	var items int
	must(t, testDB.QueryRow("SELECT COUNT(*) FROM order_items WHERE order_id = ?", order.ID).Scan(&items))
	if items != 2 {
		t.Fatalf("esperava 2 itens no pedido, veio %d", items)
	}
}

func TestCheckoutRollsBackWhenStockChanged(t *testing.T) {
	s := requireDB(t)
	ctx := context.Background()
	p := createProduct(t, s, "Checkout Estoque", 30, 2)
	sess := newSession(t)
	must(t, s.AddToCart(ctx, sess, p.ID, 2))

	// Outro cliente comprou antes: o estoque caiu depois que o item foi para o carrinho.
	_, err := s.UpdateProduct(ctx, p.ID, store.ProductInput{Name: p.Name, Brand: p.Brand, Price: p.Price, Stock: 1, PartType: p.PartType})
	must(t, err)

	if _, err := s.Checkout(ctx, sess, "credito"); !errors.Is(err, store.ErrOutOfStock) {
		t.Fatalf("esperava ErrOutOfStock, veio %v", err)
	}
	if c, _ := s.GetCart(ctx, sess); c.TotalItems != 2 {
		t.Fatalf("o carrinho deveria continuar intacto após o rollback: %+v", c)
	}
	var orders int
	must(t, testDB.QueryRow("SELECT COUNT(*) FROM orders WHERE session_id = ?", sess).Scan(&orders))
	if orders != 0 {
		t.Fatalf("nenhum pedido deveria ter sido criado, veio %d", orders)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
