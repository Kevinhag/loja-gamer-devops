<script>
  import SearchBox from "./SearchBox.svelte";
  import { link } from "../lib/router.svelte.js";
  import { cart } from "../lib/cart.svelte.js";

  let { search = false } = $props();
</script>

<header>
  <nav class="nav">
    <div class="nav-container">
      <div class="navLogo">
        <a href="/" use:link><img src="/img/logo.png" alt="K.O Gaming Store" /></a>
      </div>
      {#if search}
        <SearchBox />
      {:else}
        <div class="nav-search"></div>
      {/if}
      <div>
        <div class="navCart">
          <a href="/carrinho" use:link aria-label="Carrinho com {cart.total_items} itens">
            <i class="fa-solid fa-cart-shopping" class:rainbow={cart.total_items > 0}></i>
            {#if cart.total_items > 0}<span class="cart-badge">{cart.total_items}</span>{/if}
          </a>
        </div>
      </div>
      <div class="navLogin">
        <a href="/admin" use:link>Admin</a>
      </div>
    </div>
  </nav>
</header>
