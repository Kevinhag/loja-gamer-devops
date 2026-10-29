// Notificações rápidas (substituem os alert() da versão antiga).
export const toasts = $state([]);

let nextId = 1;

export function notify(message, type = "ok", timeout = 3500) {
  const id = nextId++;
  toasts.push({ id, message, type });
  setTimeout(() => dismiss(id), timeout);
  return id;
}

export function dismiss(id) {
  const i = toasts.findIndex((t) => t.id === id);
  if (i !== -1) toasts.splice(i, 1);
}
