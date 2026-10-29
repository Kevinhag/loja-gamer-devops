// Roteador mínimo baseado na History API. O backend Go devolve o index.html para qualquer
// rota desconhecida, então /carrinho, /checkout e /admin funcionam também ao recarregar a página.

export const route = $state({ path: window.location.pathname });

export function navigate(to) {
  if (to === route.path) return;
  history.pushState({}, "", to);
  route.path = to;
  window.scrollTo(0, 0);
}

window.addEventListener("popstate", () => {
  route.path = window.location.pathname;
});

// use:link nos <a> internos: navega sem recarregar a página.
export function link(node) {
  function onClick(event) {
    if (event.defaultPrevented || event.button !== 0) return;
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || node.target) return;
    const url = new URL(node.href);
    if (url.origin !== window.location.origin) return;
    event.preventDefault();
    navigate(url.pathname);
  }
  node.addEventListener("click", onClick);
  return {
    destroy: () => node.removeEventListener("click", onClick),
  };
}
