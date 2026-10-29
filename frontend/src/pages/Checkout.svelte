<script>
  import CartItem from "../components/CartItem.svelte";
  import { api } from "../lib/api.js";
  import { cart, clearCart, loadCart } from "../lib/cart.svelte.js";
  import { formatPrice } from "../lib/format.js";
  import { link } from "../lib/router.svelte.js";
  import { notify } from "../lib/toast.svelte.js";

  const methods = [
    { value: "pix", label: "Pix" },
    { value: "credito", label: "Crédito" },
    { value: "debito", label: "Débito" },
  ];

  let payment = $state("pix");
  let submitting = $state(false);
  let order = $state(null);

  async function finish() {
    submitting = true;
    try {
      order = await api.checkout(payment);
      clearCart();
    } catch (e) {
      notify(e.message, "error");
      // O estoque pode ter mudado: recarrega para mostrar a situação real.
      await loadCart().catch(() => {});
    } finally {
      submitting = false;
    }
  }
</script>

<main class="cartMain">
  <div class="checkout-container">
    <div class="cards-checkout">
      {#if order}
        <div class="checkout-total">
          <i class="fa-solid fa-circle-check order-ok"></i>
          <h2>Pedido #{order.id} confirmado!</h2>
          <p>{order.total_items} {order.total_items === 1 ? "item" : "itens"} · {formatPrice(order.total)}</p>
          <p>Pagamento: {methods.find((m) => m.value === order.payment_method)?.label}</p>
          <div class="admin-btn"><a href="/" use:link>Voltar para a loja</a></div>
        </div>
      {:else if cart.loaded && cart.items.length === 0}
        <div class="checkout-total">
          <h2>Seu carrinho está vazio</h2>
          <div class="admin-btn"><a href="/" use:link>Ver produtos</a></div>
        </div>
      {:else}
        <div class="cart">
          {#each cart.items as item (item.product.id)}
            <CartItem {item} readonly />
          {/each}
        </div>
        <form
          class="checkout-total"
          onsubmit={(e) => {
            e.preventDefault();
            finish();
          }}
        >
          <h2>Resumo da Compra</h2>
          <p>Total de Itens: <span data-testid="total-items">{cart.total_items}</span></p>
          <p>Valor Total: <span data-testid="total-price">{formatPrice(cart.total_price)}</span></p>
          <label for="payment"><h3>Forma de pagamento:</h3></label>
          <select id="payment" class="pDD" bind:value={payment}>
            {#each methods as m (m.value)}
              <option value={m.value}>{m.label}</option>
            {/each}
          </select>
          <div class="admin-btn">
            <button type="submit" disabled={submitting || !cart.loaded}>
              {submitting ? "Processando..." : "Finalizar Compra"}
            </button>
          </div>
        </form>
      {/if}
    </div>
  </div>
</main>
