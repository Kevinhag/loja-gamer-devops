<script>
  import ProductCard from "./ProductCard.svelte";
  import { api } from "../lib/api.js";
  import { addToCart } from "../lib/cart.svelte.js";
  import { notify } from "../lib/toast.svelte.js";

  let term = $state("");
  let open = $state(false);
  let products = $state([]);
  let loading = null;

  const results = $derived.by(() => {
    const t = term.trim().toLowerCase();
    if (!t) return [];
    return products
      .filter((p) => p.name.toLowerCase().includes(t) || (p.brand ?? "").toLowerCase().includes(t))
      .slice(0, 8);
  });

  // Carrega a lista só quando o usuário começa a buscar.
  function ensureLoaded() {
    loading ??= api
      .listProducts()
      .then((list) => {
        products = list;
      })
      .catch(() => {
        loading = null;
        notify("Não foi possível carregar a busca", "error");
      });
  }

  // Fecha só quando o foco sai da busca inteira. No código antigo o blur do input apagava
  // os resultados antes do clique no "+" chegar, então o botão nunca funcionava.
  function onFocusOut(event) {
    if (!event.currentTarget.contains(event.relatedTarget)) open = false;
  }

  async function add(product) {
    try {
      await addToCart(product.id);
      notify(`${product.name} adicionado ao carrinho!`);
    } catch (e) {
      notify(e.message, "error");
    }
  }
</script>

<!-- Esc fecha os resultados; o foco continua no input ou nos botões de dentro -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div class="search-wrap" onfocusout={onFocusOut} onkeydown={(e) => e.key === "Escape" && (open = false)} role="search">
  <div class="nav-search">
    <i class="fa-solid fa-magnifying-glass"></i>
    <input
      type="search"
      placeholder="Busca"
      aria-label="Buscar produtos"
      bind:value={term}
      onfocus={() => {
        ensureLoaded();
        open = true;
      }}
      oninput={() => (open = true)}
    />
  </div>
  {#if open && results.length > 0}
    <div class="search-container">
      {#each results as product (product.id)}
        <ProductCard {product} variant="search" onadd={add} />
      {/each}
    </div>
  {/if}
</div>
