<script lang="ts">
  import { rem } from "$lib/utils";
  import { onMount } from "svelte";
  import { TabulatorFull as Tabulator, type ColumnDefinition } from "tabulator-tables";

  export let data: any[];
  export let columns: ColumnDefinition[];
  export let tree = false;
  export let layout: "fitDataTable" | "fitColumns" = "fitDataTable";

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
        renderVertical: layout === "fitDataTable" ? "basic" : "virtual",
        renderHorizontal: layout === "fitDataTable" ? "basic" : "virtual",
        layout
      });
    }
  }

  onMount(() => {
    build();
  });
</script>

<div class="overflow-x-auto box py-0" style="max-width: 100%;" bind:this={tableComponent}></div>
