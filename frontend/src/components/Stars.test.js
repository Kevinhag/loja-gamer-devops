import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/svelte";
import Stars from "./Stars.svelte";

function count(container, kind) {
  return container.querySelectorAll(`[data-star="${kind}"]`).length;
}

describe("Stars", () => {
  it("renderiza estrelas cheias e vazias", () => {
    const { container } = render(Stars, { rating: 3 });
    expect(count(container, "filled")).toBe(3);
    expect(count(container, "empty")).toBe(2);
    expect(screen.getByRole("img", { name: "3 de 5 estrelas" })).toBeInTheDocument();
  });

  it("nota máxima usa o efeito arco-íris", () => {
    const { container } = render(Stars, { rating: 5 });
    expect(container.querySelectorAll(".rainbow")).toHaveLength(5);
  });

  it("não quebra com nota inválida", () => {
    const { container } = render(Stars, { rating: 42 });
    expect(count(container, "filled")).toBe(5);
    expect(count(container, "empty")).toBe(0);
  });
});
