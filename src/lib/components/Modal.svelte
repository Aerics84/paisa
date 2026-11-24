<script lang="ts">
  interface Props {
    active?: boolean;
    width?: string;
    bodyClass?: string;
    headerClass?: string;
    footerClass?: string;
    role?: string;
    ariaModal?: "true" | "false";
    ariaLabelledby?: string;
    ariaLabel?: string;
    ariaDescribedby?: string;
    dialogElement?: HTMLElement | null;
  }

  let {
    active = $bindable(false),
    width = "min(640px, 100vw)",
    bodyClass = "",
    headerClass = "",
    footerClass = "",
    role = undefined,
    ariaModal = undefined,
    ariaLabelledby = undefined,
    ariaLabel = undefined,
    ariaDescribedby = undefined,
    dialogElement = $bindable(null)
  }: Props = $props();

  function close(_e: unknown) {
    active = !active;
  }
</script>

<div class="modal" class:is-active={active}>
  <div
    class="modal-background"
    role="button"
    tabindex="0"
    aria-label="Close modal"
    onclick={(e) => close(e)}
    onkeydown={(e) => (e.key === "Enter" || e.key === " ") && close(e)}
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
      <!-- svelte-ignore slot_element_deprecated -->
      <slot name="head" {close} />
    </header>
    <section class="modal-card-body {bodyClass}">
      <!-- svelte-ignore slot_element_deprecated -->
      <slot name="body" />
    </section>
    <footer class="modal-card-foot {footerClass}">
      <!-- svelte-ignore slot_element_deprecated -->
      <slot name="foot" {close} />
    </footer>
  </div>
</div>
