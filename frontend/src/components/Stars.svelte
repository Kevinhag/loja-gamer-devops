<script>
  import { starParts } from "../lib/format.js";

  let { rating } = $props();
  const parts = $derived(starParts(rating));
</script>

<div class="rating-stars" role="img" aria-label="{parts.filled} de 5 estrelas">
  {#each { length: parts.filled }, i (i)}
    {#if parts.rainbow}
      <i class="fa-solid fa-star rainbow" data-star="filled"></i>
    {:else}
      <i class="fa-solid fa-star" style="color: #ffd500" data-star="filled"></i>
    {/if}
  {/each}
  {#each { length: parts.empty }, i (i)}
    <i class="fa-regular fa-star" style="color: var(--border)" data-star="empty"></i>
  {/each}
</div>
