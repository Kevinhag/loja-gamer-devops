<script>
  import { onMount } from "svelte";
  import { api, getAdminToken, setAdminToken } from "../lib/api.js";
  import { partTypeLabel, partTypeLabels } from "../lib/format.js";
  import { notify } from "../lib/toast.svelte.js";

  let checking = $state(true);
  let authed = $state(false);
  let tokenInput = $state("");

  let mode = $state("edit"); // "edit" (incluir/editar) | "delete" (excluir)
  let products = $state([]);
  let selectedId = $state("");
  let form = $state(emptyForm());
  let file = $state(null);
  let preview = $state("");
  let fileInput = $state();
  let errors = $state({});
  let saving = $state(false);

  const selected = $derived(products.find((p) => String(p.id) === selectedId));
  const partTypes = $derived(
    [...new Set([...Object.keys(partTypeLabels), ...products.map((p) => p.part_type)])].sort(),
  );

  function emptyForm() {
    return {
      name: "", brand: "", description: "", manufacturer: "", model: "", product_line: "",
      price: "", stock: "", rating: "", is_new: false, on_sale: false, part_type: "",
    };
  }

  function fromProduct(p) {
    return {
      name: p.name, brand: p.brand, description: p.description, manufacturer: p.manufacturer,
      model: p.model, product_line: p.product_line, price: String(p.price), stock: String(p.stock),
      rating: String(p.rating), is_new: p.is_new, on_sale: p.on_sale, part_type: p.part_type,
    };
  }

  onMount(async () => {
    if (getAdminToken()) await verify();
    checking = false;
  });

  async function verify() {
    try {
      await api.checkAdmin();
      authed = true;
      await loadProducts();
    } catch (e) {
      authed = false;
      setAdminToken("");
      notify(e.status === 401 ? "Token de admin inválido" : e.message, "error");
    }
  }

  async function login(event) {
    event.preventDefault();
    setAdminToken(tokenInput.trim());
    tokenInput = "";
    await verify();
  }

  function logout() {
    setAdminToken("");
    authed = false;
    select("");
  }

  async function loadProducts() {
    products = await api.listProducts();
  }

  function select(id) {
    selectedId = id;
    errors = {};
    clearFile();
    const p = products.find((p) => String(p.id) === id);
    form = p ? fromProduct(p) : emptyForm();
  }

  function setMode(m) {
    mode = m;
    select(selectedId);
  }

  function clearFile() {
    if (preview) URL.revokeObjectURL(preview);
    preview = "";
    file = null;
    if (fileInput) fileInput.value = "";
  }

  function onFile(event) {
    const f = event.currentTarget.files?.[0] ?? null;
    if (preview) URL.revokeObjectURL(preview);
    file = f;
    preview = f ? URL.createObjectURL(f) : "";
  }

  async function save(event) {
    event.preventDefault();
    saving = true;
    errors = {};
    const data = new FormData();
    for (const [k, v] of Object.entries(form)) data.set(k, String(v));
    if (file) data.set("image", file);
    try {
      const p = selected ? await api.updateProduct(selected.id, data) : await api.createProduct(data);
      notify(selected ? "Produto atualizado!" : "Produto cadastrado!");
      await loadProducts();
      select(String(p.id));
    } catch (e) {
      errors = e.fields ?? {};
      notify(e.message, "error");
      if (e.status === 401) logout();
    } finally {
      saving = false;
    }
  }

  async function remove() {
    if (!selected || !confirm(`Remover "${selected.name}"? Essa ação não pode ser desfeita.`)) return;
    try {
      await api.deleteProduct(selected.id);
      notify("Produto removido.");
      await loadProducts();
      select("");
    } catch (e) {
      notify(e.message, "error");
      if (e.status === 401) logout();
    }
  }
</script>

{#snippet field(name, label, type = "text", attrs = {})}
  <div>
    <label for="fp-{name}">{label}: </label>
    <input id="fp-{name}" {type} bind:value={form[name]} aria-invalid={!!errors[name]} {...attrs} />
    {#if errors[name]}<span class="field-error">{errors[name]}</span>{/if}
  </div>
{/snippet}

<main class="admin">
  {#if checking}
    <p class="page-message"><i class="fa-solid fa-spinner fa-spin"></i></p>
  {:else if !authed}
    <section>
      <div class="section-container">
        <form class="admin-login" onsubmit={login}>
          <img src="/img/logo.png" class="adminLogo" alt="" />
          <label for="admin-token">Token de administrador</label>
          <input id="admin-token" type="password" class="pDD" bind:value={tokenInput} autocomplete="current-password" required />
          <div class="admin-btn"><button type="submit">Entrar</button></div>
        </form>
      </div>
    </section>
  {:else}
    <div class="admin-base">
      <button type="button" class="admin-tab" class:active={mode === "edit"} onclick={() => setMode("edit")}>Incluir / Editar</button>
      /
      <button type="button" class="admin-tab" class:active={mode === "delete"} onclick={() => setMode("delete")}>Excluir</button>
      <button type="button" class="admin-tab admin-logout" onclick={logout}>Sair</button>
    </div>
    <section>
      <div class="section-container">
        <div class="admin-product-select">
          <img src="/img/logo.png" class="adminLogo" alt="" />
          <select class="pDD" aria-label="Produto" value={selectedId} onchange={(e) => select(e.currentTarget.value)}>
            <option value="">{mode === "edit" ? "-- Novo produto --" : "-- Selecione o produto --"}</option>
            {#each products as p (p.id)}
              <option value={String(p.id)}>{partTypeLabel(p.part_type)} | {p.name}</option>
            {/each}
          </select>
        </div>
        <div class="admin-product-img">
          {#if preview}
            <img src={preview} alt="Pré-visualização da nova imagem" />
          {:else if selected?.image_url}
            <img src={selected.image_url} alt={selected.name} />
          {:else}
            <i class="fa-solid fa-image image-placeholder"></i>
          {/if}
        </div>
        <form class="admin-form" onsubmit={save}>
          {#if mode === "edit"}
            <div>
              <input type="file" accept="image/png,image/jpeg,image/webp" aria-label="Imagem do produto" onchange={onFile} bind:this={fileInput} />
              {#if errors.image}<span class="field-error">{errors.image}</span>{/if}
            </div>
          {/if}
          <fieldset class="admin-product-form" disabled={mode === "delete"}>
            <div>
              <div class="form-add">
                {@render field("name", "Nome", "text", { required: true, maxlength: 255 })}
                {@render field("brand", "Marca")}
                {@render field("description", "Descrição")}
                {@render field("manufacturer", "Fabricante")}
                {@render field("model", "Modelo")}
                {@render field("product_line", "Linha")}
              </div>
            </div>
            <div>
              <div class="form-add">
                {@render field("price", "Preço", "text", { required: true, inputmode: "decimal", placeholder: "0,00" })}
                {@render field("stock", "Estoque", "number", { required: true, min: 0, step: 1 })}
                {@render field("rating", "Avaliação", "number", { min: 0, max: 5, step: 1 })}
                {@render field("part_type", "Tipo", "text", { required: true, list: "part-types" })}
                <datalist id="part-types">
                  {#each partTypes as t (t)}<option value={t}>{partTypeLabel(t)}</option>{/each}
                </datalist>
                <div class="checkbox-row">
                  <label><input type="checkbox" bind:checked={form.is_new} /> Novo</label>
                  <label><input type="checkbox" bind:checked={form.on_sale} /> Em promoção</label>
                </div>
              </div>
            </div>
          </fieldset>
          <div class="admin-btn">
            {#if mode === "edit"}
              <button type="submit" disabled={saving}>
                {saving ? "Salvando..." : selected ? "Salvar alterações" : "Adicionar produto"}
              </button>
            {:else}
              <button type="button" onclick={remove} disabled={!selected}>Remover</button>
            {/if}
          </div>
        </form>
      </div>
    </section>
  {/if}
</main>
