package api

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"lojagamer/internal/store"
)

// fakeStore é uma implementação em memória de store.Store para os testes da API.
type fakeStore struct {
	mu       sync.Mutex
	products map[int64]store.Product
	carts    map[string]map[int64]int
	nextID   int64
	orders   []store.Order
	pingErr  error
	failWith error // quando definido, todas as operações devolvem este erro
}

func newFakeStore(products ...store.Product) *fakeStore {
	f := &fakeStore{products: map[int64]store.Product{}, carts: map[string]map[int64]int{}}
	for _, p := range products {
		f.nextID++
		if p.ID == 0 {
			p.ID = f.nextID
		}
		f.products[p.ID] = p
	}
	return f
}

func (f *fakeStore) Ping(context.Context) error { return f.pingErr }

func (f *fakeStore) ListProducts(_ context.Context, flt store.ProductFilter) ([]store.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failWith != nil {
		return nil, f.failWith
	}
	out := []store.Product{}
	for _, p := range f.products {
		if flt.OnSale != nil && p.OnSale != *flt.OnSale {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if flt.Limit > 0 && len(out) > flt.Limit {
		out = out[:flt.Limit]
	}
	return out, nil
}

func (f *fakeStore) GetProduct(_ context.Context, id int64) (store.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.products[id]
	if !ok {
		return store.Product{}, store.ErrNotFound
	}
	return p, nil
}

func fromInput(id int64, in store.ProductInput) store.Product {
	return store.Product{ID: id, Name: in.Name, Brand: in.Brand, Price: in.Price, Stock: in.Stock,
		Description: in.Description, Rating: in.Rating, IsNew: in.IsNew, OnSale: in.OnSale,
		PartType: in.PartType, Manufacturer: in.Manufacturer, ProductLine: in.ProductLine, Model: in.Model,
		CreatedAt: time.Now()}
}

func (f *fakeStore) CreateProduct(_ context.Context, in store.ProductInput) (store.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	p := fromInput(f.nextID, in)
	f.products[p.ID] = p
	return p, nil
}

func (f *fakeStore) UpdateProduct(_ context.Context, id int64, in store.ProductInput) (store.Product, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	old, ok := f.products[id]
	if !ok {
		return store.Product{}, store.ErrNotFound
	}
	p := fromInput(id, in)
	p.ImageKey = old.ImageKey
	f.products[id] = p
	return p, nil
}

func (f *fakeStore) SetProductImage(_ context.Context, id int64, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.products[id]
	if !ok {
		return store.ErrNotFound
	}
	p.ImageKey = key
	f.products[id] = p
	return nil
}

func (f *fakeStore) DeleteProduct(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.products[id]; !ok {
		return store.ErrNotFound
	}
	delete(f.products, id)
	for _, c := range f.carts {
		delete(c, id)
	}
	return nil
}

func (f *fakeStore) GetCart(_ context.Context, session string) (store.Cart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cart := store.Cart{Items: []store.CartItem{}}
	ids := make([]int64, 0, len(f.carts[session]))
	for id := range f.carts[session] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		qty := f.carts[session][id]
		p := f.products[id]
		cart.Items = append(cart.Items, store.CartItem{Product: p, Quantity: qty, Subtotal: p.Price * float64(qty)})
		cart.TotalItems += qty
		cart.TotalPrice += p.Price * float64(qty)
	}
	return cart, nil
}

func (f *fakeStore) AddToCart(_ context.Context, session string, id int64, qty int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.products[id]
	if !ok {
		return store.ErrNotFound
	}
	if f.carts[session] == nil {
		f.carts[session] = map[int64]int{}
	}
	if f.carts[session][id]+qty > p.Stock {
		return &store.StockError{ProductID: id, Name: p.Name, Available: p.Stock}
	}
	f.carts[session][id] += qty
	return nil
}

func (f *fakeStore) SetCartQuantity(ctx context.Context, session string, id int64, qty int) error {
	if qty == 0 {
		return f.RemoveFromCart(ctx, session, id)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.carts[session][id]; !ok {
		return store.ErrNotFound
	}
	if p := f.products[id]; qty > p.Stock {
		return &store.StockError{ProductID: id, Name: p.Name, Available: p.Stock}
	}
	f.carts[session][id] = qty
	return nil
}

func (f *fakeStore) RemoveFromCart(_ context.Context, session string, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.carts[session][id]; !ok {
		return store.ErrNotFound
	}
	delete(f.carts[session], id)
	return nil
}

func (f *fakeStore) Checkout(_ context.Context, session, method string) (store.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failWith != nil {
		return store.Order{}, f.failWith
	}
	if len(f.carts[session]) == 0 {
		return store.Order{}, store.ErrEmptyCart
	}
	o := store.Order{ID: int64(len(f.orders) + 1), PaymentMethod: method, CreatedAt: time.Now()}
	for id, qty := range f.carts[session] {
		p := f.products[id]
		if qty > p.Stock {
			return store.Order{}, &store.StockError{ProductID: id, Name: p.Name, Available: p.Stock}
		}
		o.Total += p.Price * float64(qty)
		o.TotalItems += qty
	}
	for id, qty := range f.carts[session] {
		p := f.products[id]
		p.Stock -= qty
		f.products[id] = p
	}
	delete(f.carts, session)
	f.orders = append(f.orders, o)
	return o, nil
}

var errDB = errors.New("conexão recusada")
