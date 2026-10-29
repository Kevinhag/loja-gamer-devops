<script>
  import { onMount } from "svelte";
  import PromoBanner from "../components/PromoBanner.svelte";
  import ProductCard from "../components/ProductCard.svelte";
  import { api } from "../lib/api.js";
  import { addToCart } from "../lib/cart.svelte.js";
  import { notify } from "../lib/toast.svelte.js";

  let sections = $state([
    { id: "cards-promotion", title: "Promoções", params: { on_sale: true, random: true, limit: 5 }, products: [] },
    { id: "cards-common", title: "Destaques", params: { on_sale: false, random: true, limit: 5 }, products: [] },
    { id: "cards-all", title: "Você também pode gostar", params: { random: true, limit: 5 }, products: [] },
  ]);
  let status = $state("loading");

  onMount(async () => {
    try {
      const lists = await Promise.all(sections.map((s) => api.listProducts(s.params)));
      lists.forEach((list, i) => {
        sections[i].products = list;
      });
      status = "ready";
    } catch (e) {
      status = "error";
      notify(e.message, "error");
    }
  });

  async function add(product) {
    try {
      await addToCart(product.id);
      notify(`${product.name} adicionado ao carrinho!`);
    } catch (e) {
      notify(e.message, "error");
    }
  }
</script>

<main>
  <PromoBanner />

  {#if status === "loading"}
    <p class="page-message"><i class="fa-solid fa-spinner fa-spin"></i> Carregando produtos...</p>
  {:else if status === "error"}
    <p class="page-message">Não foi possível carregar os produtos. Tente novamente em instantes.</p>
  {:else}
    {#each sections as section (section.id)}
      {#if section.products.length > 0}
        <h2 class="section-title">{section.title}</h2>
        <section class="cards-main" id={section.id}>
          {#each section.products as product (product.id)}
            <ProductCard {product} onadd={add} />
          {/each}
        </section>
      {/if}
    {/each}
  {/if}
</main>
