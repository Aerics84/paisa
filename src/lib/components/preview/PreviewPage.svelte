<script lang="ts">
  export let title = "";
  export let eyebrow = "";
  export let description = "";
</script>

<section class="paisa-preview-page">
  <header class="paisa-preview-page__header">
    <div>
      {#if eyebrow}
        <div class="paisa-kicker">{eyebrow}</div>
      {/if}
      <h1>{title}</h1>
      {#if description}
        <p>{description}</p>
      {/if}
    </div>

    {#if $$slots.actions}
      <div class="paisa-preview-page__actions">
        <slot name="actions" />
      </div>
    {/if}
  </header>

  <div class="paisa-preview-page__body">
    <slot />
  </div>
</section>
