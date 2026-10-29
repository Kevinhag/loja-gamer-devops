<script>
  import { onMount } from "svelte";
  import Nav from "./components/Nav.svelte";
  import Toasts from "./components/Toasts.svelte";
  import Home from "./pages/Home.svelte";
  import Cart from "./pages/Cart.svelte";
  import Checkout from "./pages/Checkout.svelte";
  import Admin from "./pages/Admin.svelte";
  import NotFound from "./pages/NotFound.svelte";
  import { route } from "./lib/router.svelte.js";
  import { loadCart } from "./lib/cart.svelte.js";
  import { parallax } from "./lib/parallax.js";
  import { notify } from "./lib/toast.svelte.js";

  const routes = {
    "/": { page: Home, title: "K.O Gaming Store" },
    "/carrinho": { page: Cart, title: "Carrinho" },
    "/checkout": { page: Checkout, title: "Finalizar compra" },
    "/admin": { page: Admin, title: "Admin" },
  };
  const current = $derived(routes[route.path] ?? { page: NotFound, title: "Página não encontrada" });

  $effect(() => {
    document.title = route.path === "/" ? current.title : `${current.title} · K.O Gaming Store`;
  });

  onMount(() => {
    loadCart().catch((e) => notify(e.message, "error"));
  });
</script>

<div class="parallax-bg" use:parallax>
  <Nav search={route.path === "/"} />
  <current.page />
  <footer>©2023 - Kevin Henriques</footer>
</div>
<Toasts />
