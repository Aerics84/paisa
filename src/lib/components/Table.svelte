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
      tabulator.replaceData(data);
      // fitDataTable sizes columns from the data. replaceData() does not
      // recalculate those widths, so the header would keep the old widths
      // while body cells grow with longer content.
      tabulator.redraw(true);
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
        layout: "fitDataTable"
      });
      requestAnimationFrame(() => tabulator?.redraw(true));
    }
  }

  onMount(() => {
    build();

    // Recalculate column widths when the surrounding layout changes, otherwise
    // a late container resize leaves the header and body columns out of sync.
    let lastWidth = tableComponent.clientWidth;
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width ?? 0;
      if (width !== lastWidth) {
        lastWidth = width;
        tabulator?.redraw(true);
      }
    });
    observer.observe(tableComponent);
    return () => observer.disconnect();
  });
</script>

<div class="overflow-x-auto box py-0" style="max-width: 100%;" bind:this={tableComponent}></div>
