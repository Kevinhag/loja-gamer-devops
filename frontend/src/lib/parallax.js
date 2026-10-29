// Efeito parallax do fundo (antigo js/bgMain.js), como action do Svelte: use:parallax
export function parallax(node, speed = 0.4) {
  function onScroll() {
    node.style.backgroundPositionY = `${-window.scrollY * speed}px`;
  }
  window.addEventListener("scroll", onScroll, { passive: true });
  onScroll();
  return {
    destroy: () => window.removeEventListener("scroll", onScroll),
  };
}
