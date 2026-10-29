import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/svelte";
import CartItem from "./CartItem.svelte";

const item = {
  product: { id: 9, name: "Ryzen 7 5800X", brand: "AMD", price: 1999.99, stock: 3, rating: 4, on_sale: true, image_url: "" },
  quantity: 2,
  subtotal: 3999.98,
};

describe("CartItem", () => {
  it("mostra subtotal e preço unitário", () => {
    render(CartItem, { item });
    expect(screen.getByText(/3\.999,98/)).toBeInTheDocument();
    expect(screen.getByText(/2 × R\$\s1\.999,99/)).toBeInTheDocument();
  });

  it("altera a quantidade e remove", async () => {
    const onquantity = vi.fn();
    const onremove = vi.fn();
    render(CartItem, { item, onquantity, onremove });

    await fireEvent.click(screen.getByRole("button", { name: /aumentar/i }));
    expect(onquantity).toHaveBeenLastCalledWith(9, 3);
    await fireEvent.click(screen.getByRole("button", { name: /diminuir/i }));
    expect(onquantity).toHaveBeenLastCalledWith(9, 1);
    await fireEvent.click(screen.getByRole("button", { name: "Remover" }));
    expect(onremove).toHaveBeenCalledWith(9);
  });

  it("não deixa passar do estoque", () => {
    render(CartItem, { item: { ...item, quantity: 3 } });
    expect(screen.getByRole("button", { name: /aumentar/i })).toBeDisabled();
  });

  it("modo somente leitura (checkout) não tem controles", () => {
    render(CartItem, { item, readonly: true });
    expect(screen.queryByRole("button")).toBeNull();
    expect(screen.getByText("Qtd: 2")).toBeInTheDocument();
  });
});
