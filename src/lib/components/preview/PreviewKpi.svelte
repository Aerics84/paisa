<script lang="ts">
  export let label = "";
  export let value = "";
  export let detail = "";
  export let tone: "neutral" | "positive" | "negative" | "warning" | "accent" = "neutral";
</script>

<div
  class="paisa-preview-kpi"
  class:is-positive={tone === "positive"}
  class:is-negative={tone === "negative"}
  class:is-warning={tone === "warning"}
  class:is-accent={tone === "accent"}
>
  <span>{label}</span>
  <strong>{value}</strong>
  {#if detail}
    <small>{detail}</small>
  {/if}
</div>
