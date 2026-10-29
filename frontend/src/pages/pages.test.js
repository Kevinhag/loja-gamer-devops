import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import Home from "./Home.svelte";
import Cart from "./Cart.svelte";
import Checkout from "./Checkout.svelte";
import Admin from "./Admin.svelte";
import { cart, setCart } from "../lib/cart.svelte.js";
import { toasts } from "../lib/toast.svelte.js";

const product = (id, extra = {}) => ({
  id,
  name: `Produto ${id}`,
  brand: "Marca",
  price: 100,
  stock: 5,
  rating: 4,
  is_new: false,
  on_sale: false,
  image_url: "",
  description: "",
  manufacturer: "",
  model: "",
  product_line: "",
  part_type: "cpu",
  ...extra,
});

const cartWith = (...items) => ({
  items: items.map(([p, quantity]) => ({ product: p, quantity, subtotal: p.price * quantity })),
  total_items: items.reduce((n, [, q]) => n + q, 0),
  total_price: items.reduce((n, [p, q]) => n + p.price * q, 0),
});

// Simula o backend: cada rota "MÉTODO /caminho" devolve [status, corpo].
function mockApi(routes) {
  const fetchMock = vi.fn(async (url, init = {}) => {
    const key = `${init.method ?? "GET"} ${url.split("?")[0]}`;
    const handler = routes[key];
    if (!handler) return new Response(JSON.stringify({ error: `sem mock para ${key}` }), { status: 500 });
    const [status, body] = typeof handler === "function" ? handler(url, init) : handler;
    return new Response(body === undefined ? null : JSON.stringify(body), { status });
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

beforeEach(() => {
  sessionStorage.clear();
  toasts.splice(0);
  setCart({ items: [], total_items: 0, total_price: 0 });
  cart.loaded = false;
});

afterEach(() => vi.unstubAllGlobals());

describe("Home", () => {
  it("carrega as três vitrines e adiciona ao carrinho", async () => {
    const fetchMock = mockApi({
      "GET /api/products": (url) => [200, url.includes("on_sale=true") ? [product(1, { on_sale: true })] : [product(2)]],
      "POST /api/cart/items": [200, cartWith([product(2), 1])],
    });
    render(Home);

    expect(await screen.findByText("Promoções")).toBeInTheDocument();
    expect(screen.getByText("Produto 1")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(3);

    await fireEvent.click(screen.getAllByRole("button", { name: /adicionar ao carrinho/i })[1]);
    await waitFor(() => expect(cart.total_items).toBe(1));
    expect(toasts.at(-1).message).toBe("Produto 2 adicionado ao carrinho!");
  });

  it("mostra erro quando a API falha", async () => {
    mockApi({ "GET /api/products": [500, { error: "erro interno" }] });
    render(Home);
    expect(await screen.findByText(/não foi possível carregar os produtos/i)).toBeInTheDocument();
  });
});

describe("Cart", () => {
  it("mostra o carrinho vazio", () => {
    setCart({ items: [], total_items: 0, total_price: 0 });
    render(Cart);
    expect(screen.getByText(/seu carrinho está vazio/i)).toBeInTheDocument();
    expect(screen.queryByText("Finalizar Compra")).toBeNull();
  });

  it("remove item e atualiza o total", async () => {
    setCart(cartWith([product(1), 2]));
    mockApi({ "DELETE /api/cart/items/1": [200, cartWith()] });
    render(Cart);

    expect(screen.getByText("Finalizar Compra")).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "Remover" }));
    expect(await screen.findByText(/seu carrinho está vazio/i)).toBeInTheDocument();
  });

  it("avisa quando não há estoque para aumentar", async () => {
    setCart(cartWith([product(1, { stock: 9 }), 1]));
    mockApi({ "PUT /api/cart/items/1": [409, { error: "Estoque insuficiente para Produto 1 (disponível: 1)" }] });
    render(Cart);
    await fireEvent.click(screen.getByRole("button", { name: /aumentar/i }));
    await waitFor(() => expect(toasts.at(-1)?.type).toBe("error"));
    expect(toasts.at(-1).message).toContain("Estoque insuficiente");
  });
});

describe("Checkout", () => {
  it("finaliza o pedido com a forma de pagamento escolhida", async () => {
    setCart(cartWith([product(1), 2], [product(2, { price: 50 }), 1]));
    const fetchMock = mockApi({
      "POST /api/checkout": [201, { id: 42, total: 250, total_items: 3, payment_method: "debito" }],
    });
    render(Checkout);

    expect(screen.getByTestId("total-items")).toHaveTextContent("3");
    expect(screen.getByTestId("total-price")).toHaveTextContent("250,00");

    await fireEvent.change(screen.getByLabelText(/forma de pagamento/i), { target: { value: "debito" } });
    await fireEvent.click(screen.getByRole("button", { name: "Finalizar Compra" }));

    expect(await screen.findByText("Pedido #42 confirmado!")).toBeInTheDocument();
    expect(screen.getByText("Pagamento: Débito")).toBeInTheDocument();
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ payment_method: "debito" });
    expect(cart.total_items).toBe(0);
  });

  it("recarrega o carrinho quando o estoque acabou", async () => {
    setCart(cartWith([product(1), 2]));
    const fetchMock = mockApi({
      "POST /api/checkout": [409, { error: "Estoque insuficiente para Produto 1 (disponível: 1)" }],
      "GET /api/cart": [200, cartWith([product(1), 2])],
    });
    render(Checkout);
    await fireEvent.click(screen.getByRole("button", { name: "Finalizar Compra" }));

    await waitFor(() => expect(fetchMock).toHaveBeenCalledWith("/api/cart", expect.anything()));
    expect(toasts.at(-1).message).toContain("Estoque insuficiente");
    expect(screen.queryByText(/confirmado/)).toBeNull();
  });
});

describe("Admin", () => {
  it("pede o token e recusa token inválido", async () => {
    mockApi({ "GET /api/admin/session": [401, { error: "token de admin inválido" }] });
    render(Admin);

    await fireEvent.input(await screen.findByLabelText("Token de administrador"), { target: { value: "errado" } });
    await fireEvent.click(screen.getByRole("button", { name: "Entrar" }));

    await waitFor(() => expect(toasts.at(-1)?.message).toBe("Token de admin inválido"));
    expect(sessionStorage.getItem("lg_admin_token")).toBeNull();
  });

  it("cadastra um produto e mostra erros de validação do backend", async () => {
    const created = product(10, { name: "Novo" });
    let attempt = 0;
    const fetchMock = mockApi({
      "GET /api/admin/session": [204],
      "GET /api/products": () => [200, attempt > 1 ? [created] : []],
      "POST /api/products": () => {
        attempt++;
        return attempt === 1
          ? [400, { error: "Dados do produto inválidos", fields: { price: "preço inválido" } }]
          : [201, created];
      },
    });
    sessionStorage.setItem("lg_admin_token", "certo");
    render(Admin);

    await fireEvent.input(await screen.findByLabelText("Nome:"), { target: { value: "Novo" } });
    await fireEvent.input(screen.getByLabelText("Preço:"), { target: { value: "abc" } });
    await fireEvent.input(screen.getByLabelText("Estoque:"), { target: { value: "3" } });
    await fireEvent.input(screen.getByLabelText("Tipo:"), { target: { value: "cpu" } });
    await fireEvent.click(screen.getByRole("button", { name: "Adicionar produto" }));
    expect(await screen.findByText("preço inválido")).toBeInTheDocument();

    await fireEvent.input(screen.getByLabelText("Preço:"), { target: { value: "99,90" } });
    await fireEvent.click(screen.getByRole("button", { name: "Adicionar produto" }));
    expect(await screen.findByRole("button", { name: "Salvar alterações" })).toBeInTheDocument();

    const [, init] = fetchMock.mock.calls.findLast(([url, i]) => url === "/api/products" && i.method === "POST");
    expect(init.headers.Authorization).toBe("Bearer certo");
    expect(init.body.get("price")).toBe("99,90");
    expect(init.body.get("is_new")).toBe("false");
  });
});
