package api

import "net/http"

func (s *Server) writeCart(w http.ResponseWriter, r *http.Request, session string) {
	cart, err := s.store.GetCart(r.Context(), session)
	if err != nil {
		s.writeStoreError(w, r, err, "")
		return
	}
	for i := range cart.Items {
		cart.Items[i].Product = withImageURL(cart.Items[i].Product)
	}
	writeJSON(w, http.StatusOK, cart)
}

func (s *Server) handleGetCart(w http.ResponseWriter, r *http.Request) {
	s.writeCart(w, r, s.session(w, r))
}

func (s *Server) handleAddToCart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProductID int64 `json:"product_id"`
		Quantity  int   `json:"quantity"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Quantity == 0 {
		body.Quantity = 1
	}
	if body.ProductID <= 0 || body.Quantity < 1 || body.Quantity > 99 {
		writeError(w, http.StatusBadRequest, "product_id e quantity (1 a 99) são obrigatórios")
		return
	}
	session := s.session(w, r)
	if err := s.store.AddToCart(r.Context(), session, body.ProductID, body.Quantity); err != nil {
		s.writeStoreError(w, r, err, "Produto não encontrado")
		return
	}
	s.metrics.cartAdds.Add(float64(body.Quantity))
	s.writeCart(w, r, session)
}

func (s *Server) handleSetCartQuantity(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "productID")
	if !ok {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	var body struct {
		Quantity int `json:"quantity"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Quantity < 0 || body.Quantity > 99 {
		writeError(w, http.StatusBadRequest, "quantity deve estar entre 0 e 99")
		return
	}
	session := s.session(w, r)
	if err := s.store.SetCartQuantity(r.Context(), session, id, body.Quantity); err != nil {
		s.writeStoreError(w, r, err, "Item não está no carrinho")
		return
	}
	s.writeCart(w, r, session)
}

func (s *Server) handleRemoveFromCart(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r, "productID")
	if !ok {
		writeError(w, http.StatusBadRequest, "id inválido")
		return
	}
	session := s.session(w, r)
	if err := s.store.RemoveFromCart(r.Context(), session, id); err != nil {
		s.writeStoreError(w, r, err, "Item não está no carrinho")
		return
	}
	s.writeCart(w, r, session)
}

var paymentMethods = map[string]bool{"pix": true, "credito": true, "debito": true}

func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PaymentMethod string `json:"payment_method"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !paymentMethods[body.PaymentMethod] {
		writeError(w, http.StatusBadRequest, "Forma de pagamento inválida (pix, credito ou debito)")
		return
	}
	order, err := s.store.Checkout(r.Context(), s.session(w, r), body.PaymentMethod)
	if err != nil {
		s.metrics.checkoutFailures.Inc()
		s.writeStoreError(w, r, err, "")
		return
	}
	s.metrics.orders.WithLabelValues(order.PaymentMethod).Inc()
	s.metrics.revenue.Add(order.Total)
	s.log.Info("pedido criado", "order_id", order.ID, "total", order.Total, "items", order.TotalItems,
		"payment_method", order.PaymentMethod)
	writeJSON(w, http.StatusCreated, order)
}
