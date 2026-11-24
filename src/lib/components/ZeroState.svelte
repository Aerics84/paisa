<script lang="ts">
  import _ from "lodash";

  interface Props {
    item: any;
  }

  let { item }: Props = $props();
</script>

{#if item == null || item == false || (item != true && _.isEmpty(item))}
  <div class="has-text-centered p-6">
    <!-- svelte-ignore slot_element_deprecated -->
    <slot />
  </div>
{/if}
