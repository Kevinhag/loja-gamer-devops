import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { api, ApiError, getAdminToken, setAdminToken } from "./api.js";

function jsonResponse(status, body) {
  return Promise.resolve(
    new Response(body === undefined ? null : JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

describe("api", () => {
  let fetchMock;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    sessionStorage.clear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("monta a query string da listagem", async () => {
    fetchMock.mockReturnValue(jsonResponse(200, []));
    await api.listProducts({ on_sale: true, limit: 5, random: undefined });
    expect(fetchMock).toHaveBeenCalledWith("/api/products?on_sale=true&limit=5", expect.objectContaining({ method: "GET" }));
  });

  it("envia JSON ao adicionar no carrinho", async () => {
    fetchMock.mockReturnValue(jsonResponse(200, { items: [], total_items: 0, total_price: 0 }));
    await api.addToCart(7, 2);
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/cart/items");
    expect(init.method).toBe("POST");
    expect(init.headers["Content-Type"]).toBe("application/json");
    expect(JSON.parse(init.body)).toEqual({ product_id: 7, quantity: 2 });
  });

  it("transforma erros da API em ApiError com mensagem e campos", async () => {
    fetchMock.mockReturnValue(jsonResponse(400, { error: "Dados do produto inválidos", fields: { price: "preço inválido" } }));
    const err = await api.createProduct(new FormData()).catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(400);
    expect(err.message).toBe("Dados do produto inválidos");
    expect(err.fields).toEqual({ price: "preço inválido" });
  });

  it("usa uma mensagem padrão quando a resposta não é JSON", async () => {
    fetchMock.mockReturnValue(Promise.resolve(new Response("bad gateway", { status: 502 })));
    await expect(api.getCart()).rejects.toThrow("Erro 502");
  });

  it("informa quando o servidor está fora do ar", async () => {
    fetchMock.mockRejectedValue(new TypeError("Failed to fetch"));
    const err = await api.getCart().catch((e) => e);
    expect(err.status).toBe(0);
    expect(err.message).toBe("Não foi possível conectar ao servidor");
  });

  it("envia o token só nas rotas de admin", async () => {
    setAdminToken("abc123");
    expect(getAdminToken()).toBe("abc123");

    fetchMock.mockReturnValue(jsonResponse(204));
    await expect(api.deleteProduct(3)).resolves.toBeNull();
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe("Bearer abc123");

    fetchMock.mockReturnValue(jsonResponse(200, []));
    await api.listProducts();
    expect(fetchMock.mock.calls[1][1].headers.Authorization).toBeUndefined();

    setAdminToken("");
    expect(getAdminToken()).toBe("");
  });

  it("não define Content-Type ao enviar FormData (o navegador põe o boundary)", async () => {
    fetchMock.mockReturnValue(jsonResponse(201, { id: 1 }));
    const form = new FormData();
    form.set("name", "RTX");
    await api.createProduct(form);
    const init = fetchMock.mock.calls[0][1];
    expect(init.body).toBe(form);
    expect(init.headers["Content-Type"]).toBeUndefined();
  });
});
