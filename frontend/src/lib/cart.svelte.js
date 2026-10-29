// Estado do carrinho compartilhado entre as páginas (ícone da navbar, carrinho e checkout).
import { api } from "./api.js";

export const cart = $state({ items: [], total_items: 0, total_price: 0, loaded: false });

export function setCart(data) {
  cart.items = data.items;
  cart.total_items = data.total_items;
  cart.total_price = data.total_price;
  cart.loaded = true;
}

export async function loadCart() {
  setCart(await api.getCart());
}

export async function addToCart(productId, quantity = 1) {
  setCart(await api.addToCart(productId, quantity));
}

export async function setQuantity(productId, quantity) {
  setCart(await api.setCartQuantity(productId, quantity));
}

export async function removeFromCart(productId) {
  setCart(await api.removeFromCart(productId));
}

export function clearCart() {
  setCart({ items: [], total_items: 0, total_price: 0 });
}
