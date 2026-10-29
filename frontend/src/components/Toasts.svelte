<script>
  import { toasts, dismiss } from "../lib/toast.svelte.js";
</script>

<div class="toasts" aria-live="polite">
  {#each toasts as t (t.id)}
    <button type="button" class="toast toast-{t.type}" onclick={() => dismiss(t.id)}>
      <i class="fa-solid {t.type === 'error' ? 'fa-circle-exclamation' : 'fa-circle-check'}"></i>
      {t.message}
    </button>
  {/each}
</div>
