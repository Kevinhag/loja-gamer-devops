// Cliente da API Go. O cookie de sessão do carrinho é enviado automaticamente (mesma origem).

const TOKEN_KEY = "lg_admin_token";

export class ApiError extends Error {
  constructor(status, message, fields = {}) {
    super(message);
    this.status = status;
    this.fields = fields;
  }
}

export function getAdminToken() {
  try {
    return sessionStorage.getItem(TOKEN_KEY) ?? "";
  } catch {
    return "";
  }
}

export function setAdminToken(token) {
  try {
    if (token) sessionStorage.setItem(TOKEN_KEY, token);
    else sessionStorage.removeItem(TOKEN_KEY);
  } catch {
    // sem sessionStorage (modo privado): o token vale só até recarregar a página
  }
}

async function request(method, path, { body, form, admin = false } = {}) {
  const headers = {};
  let payload;
  if (form) {
    payload = form;
  } else if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    payload = JSON.stringify(body);
  }
  if (admin) {
    headers.Authorization = `Bearer ${getAdminToken()}`;
  }

  let res;
  try {
    res = await fetch(path, { method, headers, body: payload, credentials: "same-origin" });
  } catch {
    throw new ApiError(0, "Não foi possível conectar ao servidor");
  }
  if (res.status === 204) return null;

  const data = await res.json().catch(() => null);
  if (!res.ok) {
    throw new ApiError(res.status, data?.error ?? `Erro ${res.status}`, data?.fields ?? {});
  }
  return data;
}

function query(params) {
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null) qs.set(k, String(v));
  }
  const s = qs.toString();
  return s ? `?${s}` : "";
}

export const api = {
  listProducts: (params = {}) => request("GET", `/api/products${query(params)}`),

  getCart: () => request("GET", "/api/cart"),
  addToCart: (productId, quantity = 1) =>
    request("POST", "/api/cart/items", { body: { product_id: productId, quantity } }),
  setCartQuantity: (productId, quantity) =>
    request("PUT", `/api/cart/items/${productId}`, { body: { quantity } }),
  removeFromCart: (productId) => request("DELETE", `/api/cart/items/${productId}`),
  checkout: (paymentMethod) => request("POST", "/api/checkout", { body: { payment_method: paymentMethod } }),

  checkAdmin: () => request("GET", "/api/admin/session", { admin: true }),
  createProduct: (form) => request("POST", "/api/products", { form, admin: true }),
  updateProduct: (id, form) => request("PUT", `/api/products/${id}`, { form, admin: true }),
  deleteProduct: (id) => request("DELETE", `/api/products/${id}`, { admin: true }),
};
