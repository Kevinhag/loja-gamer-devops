// Package store contém os modelos da loja e o acesso ao MySQL.
package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound   = errors.New("não encontrado")
	ErrOutOfStock = errors.New("estoque insuficiente")
	ErrEmptyCart  = errors.New("carrinho vazio")
)

type Product struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Brand        string    `json:"brand"`
	Price        float64   `json:"price"`
	Stock        int       `json:"stock"`
	Description  string    `json:"description"`
	Rating       int       `json:"rating"`
	IsNew        bool      `json:"is_new"`
	OnSale       bool      `json:"on_sale"`
	PartType     string    `json:"part_type"`
	Manufacturer string    `json:"manufacturer"`
	ProductLine  string    `json:"product_line"`
	Model        string    `json:"model"`
	ImageKey     string    `json:"-"`
	ImageURL     string    `json:"image_url"`
	CreatedAt    time.Time `json:"created_at"`
}

// ProductInput são os campos editáveis de um produto (admin).
type ProductInput struct {
	Name         string
	Brand        string
	Price        float64
	Stock        int
	Description  string
	Rating       int
	IsNew        bool
	OnSale       bool
	PartType     string
	Manufacturer string
	ProductLine  string
	Model        string
}

type ProductFilter struct {
	OnSale *bool
	Random bool
	Limit  int
}

type CartItem struct {
	Product  Product `json:"product"`
	Quantity int     `json:"quantity"`
	Subtotal float64 `json:"subtotal"`
}

type Cart struct {
	Items      []CartItem `json:"items"`
	TotalItems int        `json:"total_items"`
	TotalPrice float64    `json:"total_price"`
}

type Order struct {
	ID            int64     `json:"id"`
	PaymentMethod string    `json:"payment_method"`
	Total         float64   `json:"total"`
	TotalItems    int       `json:"total_items"`
	CreatedAt     time.Time `json:"created_at"`
}

// StockError diz qual produto não tem estoque suficiente.
type StockError struct {
	ProductID int64
	Name      string
	Available int
}

func (e *StockError) Error() string { return "estoque insuficiente para " + e.Name }
func (e *StockError) Unwrap() error { return ErrOutOfStock }

// Store é a interface usada pela API; a implementação real é MySQL.
type Store interface {
	Ping(ctx context.Context) error

	ListProducts(ctx context.Context, f ProductFilter) ([]Product, error)
	GetProduct(ctx context.Context, id int64) (Product, error)
	CreateProduct(ctx context.Context, in ProductInput) (Product, error)
	UpdateProduct(ctx context.Context, id int64, in ProductInput) (Product, error)
	SetProductImage(ctx context.Context, id int64, key string) error
	DeleteProduct(ctx context.Context, id int64) error

	GetCart(ctx context.Context, session string) (Cart, error)
	AddToCart(ctx context.Context, session string, productID int64, qty int) error
	SetCartQuantity(ctx context.Context, session string, productID int64, qty int) error
	RemoveFromCart(ctx context.Context, session string, productID int64) error

	Checkout(ctx context.Context, session, paymentMethod string) (Order, error)
}
