<script>
  let { product } = $props();
  let failed = $state(false);
</script>

{#if product.image_url && !failed}
  <img src={product.image_url} alt={product.name} loading="lazy" onerror={() => (failed = true)} />
{:else}
  <i class="fa-solid fa-image image-placeholder" aria-label="Sem imagem"></i>
{/if}
