<script>
  // Banner com as bordas arco-íris que se afastam com o scroll (antigo trecho do js/script.js).
  const BORDERS = 8;
  const INSET = 12;
  let scrollY = $state(0);

  function borderStyle(i, y) {
    return [
      `border: ${BORDERS / 1.5 - i / 2}px solid red`,
      `left: ${(i * INSET) / 2}px`,
      `right: ${(i * INSET) / 2}px`,
      `top: ${i * INSET + (y / 20) * i}px`,
      `bottom: ${i * INSET - (y / 20) * i}px`,
      `animation-delay: ${i * -0.4}s`,
      `opacity: ${(BORDERS * 1.25 - i) / BORDERS}`,
      `z-index: ${-1 - i}`,
    ].join("; ");
  }
</script>

<svelte:window bind:scrollY />

<section class="main-banner">
  <div class="containerqqc">
    <div id="promo-banner" class="d-flex flex-align-center">
      <div class="d-flex flex-align-center logo-promo">
        <img class="logo-promo" src="/img/logo.png" alt="" />
      </div>
      <div class="d-flex flex-align-center text-promo">
        <img src="/img/banner.webp" alt="Ofertas K.O Gaming Store" />
      </div>
      {#each { length: BORDERS }, i (i)}
        <div class="borda rainbow-border" style={borderStyle(i, scrollY)}></div>
      {/each}
    </div>
  </div>
</section>
