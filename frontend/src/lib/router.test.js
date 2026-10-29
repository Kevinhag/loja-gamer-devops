import { describe, expect, it } from "vitest";
import { link, navigate, route } from "./router.svelte.js";

describe("router", () => {
  it("navigate atualiza a rota e o histórico", () => {
    navigate("/carrinho");
    expect(route.path).toBe("/carrinho");
    expect(window.location.pathname).toBe("/carrinho");
    navigate("/");
    expect(route.path).toBe("/");
  });

  it("use:link intercepta cliques em links internos", () => {
    const a = document.createElement("a");
    a.href = "/checkout";
    document.body.append(a);
    const action = link(a);

    a.click();
    expect(route.path).toBe("/checkout");

    action.destroy();
    a.remove();
    navigate("/");
  });

  it("use:link deixa passar ctrl+clique (abrir em nova aba)", () => {
    const a = document.createElement("a");
    a.href = "/admin";
    const action = link(a);
    let preventedByRouter;
    a.addEventListener("click", (e) => {
      preventedByRouter = e.defaultPrevented;
      e.preventDefault(); // evita que o jsdom tente navegar de verdade
    });
    a.dispatchEvent(new MouseEvent("click", { ctrlKey: true, cancelable: true, bubbles: true }));
    expect(preventedByRouter).toBe(false);
    expect(route.path).toBe("/");
    action.destroy();
  });
});
