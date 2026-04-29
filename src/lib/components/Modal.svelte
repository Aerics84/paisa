<script lang="ts">
  export let active = false;
  export let width = "min(640px, 100vw)";
  export let bodyClass = "";
  export let headerClass = "";
  export let footerClass = "";
  let close = function (_e: any) {
    active = !active;
  };
</script>

<div class="modal" class:is-active={active}>
  <div
    class="modal-background"
    role="button"
    tabindex="0"
    aria-label="Close modal"
    on:click={(e) => close(e)}
    on:keydown={(e) => (e.key === "Enter" || e.key === " ") && close(e)}
  ></div>
  <div class="modal-card" style:width>
    <header class="modal-card-head {headerClass}">
      <slot name="head" {close} />
    </header>
    <section class="modal-card-body {bodyClass}">
      <slot name="body" />
    </section>
    <footer class="modal-card-foot {footerClass}">
      <slot name="foot" {close} />
    </footer>
  </div>
</div>
