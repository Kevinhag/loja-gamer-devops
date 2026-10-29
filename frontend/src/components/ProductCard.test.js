import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/svelte";
import ProductCard from "./ProductCard.svelte";

const base = {
  id: 1,
  name: "GeForce RTX 3060",
  brand: "NVIDIA",
  price: 2099.99,
  stock: 5,
  rating: 4,
  is_new: false,
  on_sale: false,
  image_url: "/images/graphics_card/rtx-3060.webp",
};

describe("ProductCard", () => {
  it("mostra nome, marca, preço e estoque", () => {
    render(ProductCard, { product: base });
    expect(screen.getByText("GeForce RTX 3060")).toBeInTheDocument();
    expect(screen.getByText("NVIDIA")).toBeInTheDocument();
    expect(screen.getByText(/2\.099,99/)).toBeInTheDocument();
    expect(screen.getByText("5 em estoque")).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "GeForce RTX 3060" })).toHaveAttribute("src", base.image_url);
  });

  it("não interpreta HTML vindo do banco (proteção contra XSS)", () => {
    const { container } = render(ProductCard, { product: { ...base, name: '<img src=x onerror="alert(1)">' } });
    expect(screen.getByText('<img src=x onerror="alert(1)">')).toBeInTheDocument();
    expect(container.querySelector("img[onerror]")).toBeNull();
  });

  it("usa o estilo de promoção e o selo NEW", () => {
    const { container } = render(ProductCard, { product: { ...base, on_sale: true, is_new: true } });
    expect(container.querySelector(".promotion")).not.toBeNull();
    expect(screen.getByText("NEW")).toBeInTheDocument();
  });

  it("chama onadd ao clicar em adicionar", async () => {
    const onadd = vi.fn().mockResolvedValue();
    render(ProductCard, { product: base, onadd });
    await fireEvent.click(screen.getByRole("button", { name: /adicionar ao carrinho/i }));
    expect(onadd).toHaveBeenCalledWith(base);
  });

  it("desabilita o botão quando o produto está esgotado", async () => {
    const onadd = vi.fn();
    render(ProductCard, { product: { ...base, stock: 0 }, onadd });
    const button = screen.getByRole("button", { name: "Esgotado" });
    expect(button).toBeDisabled();
    await fireEvent.click(button);
    expect(onadd).not.toHaveBeenCalled();
  });

  it("mostra um ícone quando o produto não tem imagem", () => {
    render(ProductCard, { product: { ...base, image_url: "" } });
    expect(screen.getByLabelText("Sem imagem")).toBeInTheDocument();
  });

  it("variante de busca tem botão compacto acessível", async () => {
    const onadd = vi.fn().mockResolvedValue();
    render(ProductCard, { product: base, variant: "search", onadd });
    await fireEvent.click(screen.getByRole("button", { name: "Adicionar GeForce RTX 3060 ao carrinho" }));
    expect(onadd).toHaveBeenCalledOnce();
  });
});
