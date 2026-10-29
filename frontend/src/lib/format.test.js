import { describe, expect, it } from "vitest";
import { formatPrice, partTypeLabel, starParts, stockLabel } from "./format.js";

const nbsp = (s) => s.replace(/\s/g, " ");

describe("formatPrice", () => {
  it("formata em reais", () => {
    expect(nbsp(formatPrice(2099.99))).toBe("R$ 2.099,99");
    expect(nbsp(formatPrice(0))).toBe("R$ 0,00");
  });

  it("aceita string e valores inválidos", () => {
    expect(nbsp(formatPrice("150.5"))).toBe("R$ 150,50");
    expect(nbsp(formatPrice(undefined))).toBe("R$ 0,00");
  });
});

describe("starParts", () => {
  it("divide entre estrelas cheias e vazias", () => {
    expect(starParts(3)).toEqual({ filled: 3, empty: 2, rainbow: false });
  });

  it("usa o arco-íris só com nota 5", () => {
    expect(starParts(5)).toEqual({ filled: 5, empty: 0, rainbow: true });
  });

  it("limita valores fora de 0..5 (antes quebrava a página)", () => {
    expect(starParts(9)).toEqual({ filled: 5, empty: 0, rainbow: true });
    expect(starParts(-2)).toEqual({ filled: 0, empty: 5, rainbow: false });
    expect(starParts(null)).toEqual({ filled: 0, empty: 5, rainbow: false });
    expect(starParts("4")).toEqual({ filled: 4, empty: 1, rainbow: false });
  });
});

describe("stockLabel", () => {
  it.each([
    [0, "Esgotado"],
    [1, "Última unidade"],
    [7, "7 em estoque"],
  ])("estoque %i -> %s", (stock, label) => {
    expect(stockLabel(stock)).toBe(label);
  });
});

describe("partTypeLabel", () => {
  it("traduz tipos conhecidos e mantém os desconhecidos", () => {
    expect(partTypeLabel("graphics_card")).toBe("Placa de vídeo");
    expect(partTypeLabel("controle")).toBe("controle");
  });
});
