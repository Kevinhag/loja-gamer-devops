<script>
  import Stars from "./Stars.svelte";
  import ProductImage from "./ProductImage.svelte";
  import { formatPrice, stockLabel } from "../lib/format.js";

  // variant "main": card da vitrine; "search": linha compacta do resultado da busca
  let { product, variant = "main", onadd } = $props();

  const soldOut = $derived(product.stock <= 0);
  let busy = $state(false);

  async function add() {
    if (busy || soldOut) return;
    busy = true;
    try {
      await onadd?.(product);
    } finally {
      busy = false;
    }
  }
</script>

{#if variant === "search"}
  <div class={product.on_sale ? "promotion-cart" : "common-cart"}>
    <div class="product-search">
      {#if product.is_new}<div class="product-new">NEW</div>{/if}
      <div class="search-card-product-name">
        <div class="cart-product-name">{product.name}</div>
        <div class="cart-product-brand">{product.brand}</div>
      </div>
      <div class="search-product-img"><ProductImage {product} /></div>
      <div class="search-card-price">{formatPrice(product.price)}</div>
      <div class="search-card-rating">
        <Stars rating={product.rating} />
        <div class="rating-users">{stockLabel(product.stock)}</div>
      </div>
      <button
        type="button"
        class="search-product-btn"
        onclick={add}
        disabled={soldOut || busy}
        aria-label="Adicionar {product.name} ao carrinho">+</button
      >
    </div>
  </div>
{:else}
  <div class="{product.on_sale ? 'promotion' : 'common'} card-hover">
    <div class="product">
      {#if product.is_new}<div class="product-new">NEW</div>{/if}
      <div class="card-product-name">
        <div class="product-name">{product.name}</div>
        <div class="product-brand">{product.brand}</div>
      </div>
      <div class="product-img"><ProductImage {product} /></div>
      <div class="card-price">{formatPrice(product.price)}</div>
      <div class="card-rating">
        <Stars rating={product.rating} />
        <div class="rating-users">{stockLabel(product.stock)}</div>
      </div>
      <button type="button" class="product-btn" onclick={add} disabled={soldOut || busy}>
        {soldOut ? "Esgotado" : "Adicionar ao Carrinho"}
      </button>
    </div>
  </div>
{/if}
