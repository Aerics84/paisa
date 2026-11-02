<script lang="ts">
  export let active = false;
  export let width = "min(640px, 100vw)";
  export let bodyClass = "";
  export let headerClass = "";
  export let footerClass = "";
  export let role: string | undefined = undefined;
  export let ariaModal: "true" | "false" | undefined = undefined;
  export let ariaLabelledby: string | undefined = undefined;
  export let ariaLabel: string | undefined = undefined;
  export let ariaDescribedby: string | undefined = undefined;
  export let dialogElement: HTMLElement | null = null;
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
  <div
    class="modal-card"
    style:width
    {role}
    aria-modal={ariaModal}
    aria-labelledby={ariaLabelledby}
    aria-label={ariaLabel}
    aria-describedby={ariaDescribedby}
    tabindex="-1"
    bind:this={dialogElement}
  >
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
