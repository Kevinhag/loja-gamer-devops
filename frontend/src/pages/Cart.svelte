<script>
  import CartItem from "../components/CartItem.svelte";
  import { cart, setQuantity, removeFromCart } from "../lib/cart.svelte.js";
  import { formatPrice } from "../lib/format.js";
  import { link } from "../lib/router.svelte.js";
  import { notify } from "../lib/toast.svelte.js";

  async function changeQuantity(productId, quantity) {
    try {
      await setQuantity(productId, quantity);
    } catch (e) {
      notify(e.message, "error");
    }
  }

  async function remove(productId) {
    try {
      await removeFromCart(productId);
      notify("Produto removido do carrinho.");
    } catch (e) {
      notify(e.message, "error");
    }
  }
</script>

<main class="cartMain">
  <div class="cart-container">
    <section class="cart" aria-label="Itens do carrinho">
      {#if !cart.loaded}
        <p class="page-message"><i class="fa-solid fa-spinner fa-spin"></i> Carregando carrinho...</p>
      {:else if cart.items.length === 0}
        <p class="page-message">Seu carrinho está vazio. <a href="/" use:link>Ver produtos</a></p>
      {:else}
        {#each cart.items as item (item.product.id)}
          <CartItem {item} onquantity={changeQuantity} onremove={remove} />
        {/each}
        <div class="cart-summary-line">
          {cart.total_items} {cart.total_items === 1 ? "item" : "itens"} · <strong>{formatPrice(cart.total_price)}</strong>
        </div>
      {/if}
    </section>
    {#if cart.items.length > 0}
      <a class="checkout-btn" href="/checkout" use:link>Finalizar Compra</a>
    {/if}
  </div>
</main>
