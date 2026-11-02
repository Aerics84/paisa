<script lang="ts">
  import { rem } from "$lib/utils";
  import { onMount } from "svelte";
  import { TabulatorFull as Tabulator, type ColumnDefinition } from "tabulator-tables";

  export let data: any[];
  export let columns: ColumnDefinition[];
  export let tree = false;

  let tableComponent: HTMLElement;
  let tabulator: Tabulator;

  $: if (data.length > 0) {
    build();
  }

  async function build() {
    if (data.length === 0) {
      return;
    }

    if (tabulator) {
      // setData() performs a full re-render and re-runs the fitDataTable
      // layout. replaceData() keeps the current column widths, which leaves
      // the header too narrow once longer content arrives.
      tabulator.setData(data);
    } else {
      tabulator = new Tabulator(tableComponent, {
        dataTree: tree,
        dataTreeStartExpanded: [true, true, false],
        dataTreeBranchElement: false,
        dataTreeChildIndent: rem(30),
        dataTreeCollapseElement:
          "<span class='has-text-link icon is-small mr-3'><i class='fas fa-angle-up'></i></span>",
        dataTreeExpandElement:
          "<span class='has-text-link icon is-small mr-3'><i class='fas fa-angle-down'></i></span>",
        data: data,
        columns: columns,
        // fitDataTable measures the rendered cells. The virtual renderers only
        // mount the visible rows, so long account names further down the table
        // are never measured and the header columns stay too narrow.
        renderVertical: "basic",
        renderHorizontal: "basic",
        layout: "fitDataTable"
      });
    }
  }

  onMount(() => {
    build();
  });
</script>

<div
  class="overflow-x-auto box px-0 py-0"
  style="max-width: 100%;"
  bind:this={tableComponent}
></div>

<style lang="scss">
  /* End the table box at the last real column instead of extending it with
     Tabulator's empty filler column and a trailing strip of background. */
  :global(.tabulator) {
    width: fit-content !important;
    max-width: 100%;
  }
  :global(.tabulator .tabulator-filler) {
    display: none !important;
  }
</style>
