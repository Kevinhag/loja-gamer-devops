package store

import (
	"errors"
	"testing"
)

func TestRoundCents(t *testing.T) {
	cases := map[float64]float64{
		0.1 + 0.2:       0.3,
		2099.99 * 3:     6299.97,
		1.006:           1.01,
		-1.006:          -1.01,
		12999.99 + 0.01: 13000,
	}
	for in, want := range cases {
		if got := roundCents(in); got != want {
			t.Errorf("roundCents(%v) = %v, esperado %v", in, got, want)
		}
	}
}

func TestStockErrorUnwrapsToErrOutOfStock(t *testing.T) {
	var err error = &StockError{ProductID: 1, Name: "RTX", Available: 2}
	if !errors.Is(err, ErrOutOfStock) {
		t.Fatal("StockError deveria ser ErrOutOfStock")
	}
	if err.Error() != "estoque insuficiente para RTX" {
		t.Fatalf("mensagem inesperada: %q", err.Error())
	}
}
