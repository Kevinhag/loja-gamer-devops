const brl = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });

export function formatPrice(value) {
  return brl.format(Number(value) || 0);
}

// Quantas estrelas cheias e vazias mostrar. Valores fora de 0..5 são limitados
// (no código antigo um rating 9 quebrava a página com RangeError).
export function starParts(rating) {
  const filled = Math.max(0, Math.min(5, Math.trunc(Number(rating) || 0)));
  return { filled, empty: 5 - filled, rainbow: filled === 5 };
}

export function stockLabel(stock) {
  if (stock <= 0) return "Esgotado";
  if (stock === 1) return "Última unidade";
  return `${stock} em estoque`;
}

export const partTypeLabels = {
  graphics_card: "Placa de vídeo",
  cpu: "Processador",
  ram: "Memória RAM",
  ssd: "SSD",
  hdd: "HD",
  motherboard: "Placa-mãe",
  cooler: "Cooler",
  psu: "Fonte",
  case: "Gabinete",
  keyboard: "Teclado",
  mouse: "Mouse",
  monitor: "Monitor",
  webcam: "Webcam",
  microphone: "Microfone",
  audio_interface: "Interface de áudio",
  speakers: "Caixa de som",
};

export function partTypeLabel(type) {
  return partTypeLabels[type] ?? type;
}
