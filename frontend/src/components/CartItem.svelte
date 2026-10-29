<script>
  import Stars from "./Stars.svelte";
  import ProductImage from "./ProductImage.svelte";
  import { formatPrice } from "../lib/format.js";

  let { item, onquantity, onremove, readonly = false } = $props();
  const product = $derived(item.product);
</script>

<div class={product.on_sale ? "promotion-cart" : "common-cart"}>
  <div class="product-cart">
    <div class="cart-card-product-name">
      <div class="cart-product-name">{product.name}</div>
      <div class="cart-product-brand">{product.brand}</div>
    </div>
    <div class="cart-product-img"><ProductImage {product} /></div>
    <div class="cart-card-price">
      {formatPrice(item.subtotal)}
      {#if item.quantity > 1}<small class="unit-price">{item.quantity} × {formatPrice(product.price)}</small>{/if}
    </div>
    <div class="cart-card-rating" class:readonly>
      <Stars rating={product.rating} />
      {#if readonly}
        <div class="rating-users">Qtd: {item.quantity}</div>
      {:else}
        <div class="cart-qty">
          <button
            type="button"
            onclick={() => onquantity?.(product.id, item.quantity - 1)}
            aria-label="Diminuir quantidade de {product.name}">−</button
          >
          <span aria-live="polite">{item.quantity}</span>
          <button
            type="button"
            onclick={() => onquantity?.(product.id, item.quantity + 1)}
            disabled={item.quantity >= product.stock}
            aria-label="Aumentar quantidade de {product.name}">+</button
          >
        </div>
      {/if}
    </div>
    {#if !readonly}
      <button type="button" class="cart-product-btn" onclick={() => onremove?.(product.id)}>Remover</button>
    {/if}
  </div>
</div>
