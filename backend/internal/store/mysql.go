package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
)

type MySQL struct {
	db *sql.DB
}

var _ Store = (*MySQL)(nil)

func NewMySQL(db *sql.DB) *MySQL { return &MySQL{db: db} }

const productColumns = `p.id, p.name, p.brand, p.price, p.stock, p.description, p.rating, p.is_new, p.on_sale,
	p.part_type, p.manufacturer, p.product_line, p.model, p.image_key, p.created_at`

type scanner interface{ Scan(dest ...any) error }

func scanProduct(s scanner, extra ...any) (Product, error) {
	var p Product
	dest := []any{&p.ID, &p.Name, &p.Brand, &p.Price, &p.Stock, &p.Description, &p.Rating, &p.IsNew, &p.OnSale,
		&p.PartType, &p.Manufacturer, &p.ProductLine, &p.Model, &p.ImageKey, &p.CreatedAt}
	err := s.Scan(append(dest, extra...)...)
	return p, err
}

func (m *MySQL) Ping(ctx context.Context) error { return m.db.PingContext(ctx) }

func (m *MySQL) ListProducts(ctx context.Context, f ProductFilter) ([]Product, error) {
	var (
		where []string
		args  []any
	)
	if f.OnSale != nil {
		where = append(where, "p.on_sale = ?")
		args = append(args, *f.OnSale)
	}
	q := "SELECT " + productColumns + " FROM products p"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	if f.Random {
		q += " ORDER BY RAND()"
	} else {
		q += " ORDER BY p.part_type, p.name"
	}
	if f.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, f.Limit)
	}

	rows, err := m.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

func (m *MySQL) GetProduct(ctx context.Context, id int64) (Product, error) {
	row := m.db.QueryRowContext(ctx, "SELECT "+productColumns+" FROM products p WHERE p.id = ?", id)
	p, err := scanProduct(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Product{}, ErrNotFound
	}
	return p, err
}

func (m *MySQL) CreateProduct(ctx context.Context, in ProductInput) (Product, error) {
	res, err := m.db.ExecContext(ctx, `INSERT INTO products
		(name, brand, price, stock, description, rating, is_new, on_sale, part_type, manufacturer, product_line, model)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.Name, in.Brand, in.Price, in.Stock, in.Description, in.Rating, in.IsNew, in.OnSale,
		in.PartType, in.Manufacturer, in.ProductLine, in.Model)
	if err != nil {
		return Product{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Product{}, err
	}
	return m.GetProduct(ctx, id)
}

func (m *MySQL) UpdateProduct(ctx context.Context, id int64, in ProductInput) (Product, error) {
	_, err := m.db.ExecContext(ctx, `UPDATE products SET
		name = ?, brand = ?, price = ?, stock = ?, description = ?, rating = ?, is_new = ?, on_sale = ?,
		part_type = ?, manufacturer = ?, product_line = ?, model = ?
		WHERE id = ?`,
		in.Name, in.Brand, in.Price, in.Stock, in.Description, in.Rating, in.IsNew, in.OnSale,
		in.PartType, in.Manufacturer, in.ProductLine, in.Model, id)
	if err != nil {
		return Product{}, err
	}
	// RowsAffected é 0 quando nada mudou, então a existência é confirmada pelo SELECT.
	return m.GetProduct(ctx, id)
}

func (m *MySQL) SetProductImage(ctx context.Context, id int64, key string) error {
	res, err := m.db.ExecContext(ctx, "UPDATE products SET image_key = ? WHERE id = ?", key, id)
	if err != nil {
		return err
	}
	return m.requireExists(ctx, res, id)
}

func (m *MySQL) DeleteProduct(ctx context.Context, id int64) error {
	res, err := m.db.ExecContext(ctx, "DELETE FROM products WHERE id = ?", id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *MySQL) requireExists(ctx context.Context, res sql.Result, id int64) error {
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	_, err := m.GetProduct(ctx, id)
	return err
}

func (m *MySQL) GetCart(ctx context.Context, session string) (Cart, error) {
	rows, err := m.db.QueryContext(ctx, "SELECT "+productColumns+`, c.quantity
		FROM cart_items c JOIN products p ON p.id = c.product_id
		WHERE c.session_id = ?
		ORDER BY c.created_at, p.id`, session)
	if err != nil {
		return Cart{}, err
	}
	defer rows.Close()

	cart := Cart{Items: []CartItem{}}
	for rows.Next() {
		var qty int
		p, err := scanProduct(rows, &qty)
		if err != nil {
			return Cart{}, err
		}
		item := CartItem{Product: p, Quantity: qty, Subtotal: roundCents(p.Price * float64(qty))}
		cart.Items = append(cart.Items, item)
		cart.TotalItems += qty
		cart.TotalPrice += item.Subtotal
	}
	cart.TotalPrice = roundCents(cart.TotalPrice)
	return cart, rows.Err()
}

func (m *MySQL) AddToCart(ctx context.Context, session string, productID int64, qty int) error {
	return m.withTx(ctx, func(tx *sql.Tx) error {
		p, current, err := lockCartProduct(ctx, tx, session, productID)
		if err != nil {
			return err
		}
		if current+qty > p.Stock {
			return &StockError{ProductID: p.ID, Name: p.Name, Available: p.Stock}
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO cart_items (session_id, product_id, quantity) VALUES (?, ?, ?)
			ON DUPLICATE KEY UPDATE quantity = quantity + ?`, session, productID, qty, qty)
		return err
	})
}

func (m *MySQL) SetCartQuantity(ctx context.Context, session string, productID int64, qty int) error {
	if qty <= 0 {
		return m.RemoveFromCart(ctx, session, productID)
	}
	return m.withTx(ctx, func(tx *sql.Tx) error {
		p, current, err := lockCartProduct(ctx, tx, session, productID)
		if err != nil {
			return err
		}
		if current == 0 {
			return ErrNotFound
		}
		if qty > p.Stock {
			return &StockError{ProductID: p.ID, Name: p.Name, Available: p.Stock}
		}
		_, err = tx.ExecContext(ctx, "UPDATE cart_items SET quantity = ? WHERE session_id = ? AND product_id = ?",
			qty, session, productID)
		return err
	})
}

func (m *MySQL) RemoveFromCart(ctx context.Context, session string, productID int64) error {
	res, err := m.db.ExecContext(ctx, "DELETE FROM cart_items WHERE session_id = ? AND product_id = ?", session, productID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// lockCartProduct trava a linha do produto e devolve a quantidade que já está no carrinho.
func lockCartProduct(ctx context.Context, tx *sql.Tx, session string, productID int64) (Product, int, error) {
	var p Product
	err := tx.QueryRowContext(ctx, "SELECT id, name, stock FROM products WHERE id = ? FOR UPDATE", productID).
		Scan(&p.ID, &p.Name, &p.Stock)
	if errors.Is(err, sql.ErrNoRows) {
		return p, 0, ErrNotFound
	}
	if err != nil {
		return p, 0, err
	}
	var current int
	err = tx.QueryRowContext(ctx, "SELECT quantity FROM cart_items WHERE session_id = ? AND product_id = ?",
		session, productID).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return p, 0, err
	}
	return p, current, nil
}

// Checkout cria o pedido, baixa o estoque e esvazia o carrinho numa única transação.
func (m *MySQL) Checkout(ctx context.Context, session, paymentMethod string) (Order, error) {
	var order Order
	err := m.withTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT p.id, p.name, p.price, p.stock, c.quantity
			FROM cart_items c JOIN products p ON p.id = c.product_id
			WHERE c.session_id = ?
			ORDER BY p.id
			FOR UPDATE`, session)
		if err != nil {
			return err
		}
		type line struct {
			id         int64
			name       string
			price      float64
			stock, qty int
		}
		var lines []line
		for rows.Next() {
			var l line
			if err := rows.Scan(&l.id, &l.name, &l.price, &l.stock, &l.qty); err != nil {
				rows.Close()
				return err
			}
			lines = append(lines, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(lines) == 0 {
			return ErrEmptyCart
		}

		var total float64
		for _, l := range lines {
			if l.qty > l.stock {
				return &StockError{ProductID: l.id, Name: l.name, Available: l.stock}
			}
			total += l.price * float64(l.qty)
			order.TotalItems += l.qty
		}
		order.Total = roundCents(total)
		order.PaymentMethod = paymentMethod

		res, err := tx.ExecContext(ctx, "INSERT INTO orders (session_id, payment_method, total) VALUES (?, ?, ?)",
			session, paymentMethod, order.Total)
		if err != nil {
			return err
		}
		if order.ID, err = res.LastInsertId(); err != nil {
			return err
		}
		for _, l := range lines {
			if _, err := tx.ExecContext(ctx, `INSERT INTO order_items (order_id, product_id, product_name, unit_price, quantity)
				VALUES (?, ?, ?, ?, ?)`, order.ID, l.id, l.name, l.price, l.qty); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, "UPDATE products SET stock = stock - ? WHERE id = ?", l.qty, l.id); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM cart_items WHERE session_id = ?", session); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx, "SELECT created_at FROM orders WHERE id = ?", order.ID).Scan(&order.CreatedAt)
	})
	return order, err
}

func (m *MySQL) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w (rollback: %v)", err, rbErr)
		}
		return err
	}
	return tx.Commit()
}

func roundCents(v float64) float64 { return math.Round(v*100) / 100 }
