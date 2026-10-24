<script lang="ts">
  import { rem } from "$lib/utils";
  import { onDestroy, onMount, tick } from "svelte";
  import { TabulatorFull as Tabulator, type ColumnDefinition } from "tabulator-tables";

  export let data: any[];
  export let columns: ColumnDefinition[];
  export let tree = false;
  export let preview = false;

  let tableComponent: HTMLElement;
  let tabulator: Tabulator | undefined;
  let mounted = false;
  let buildToken = 0;

  $: if (mounted) {
    data;
    columns;
    tree;
    build();
  }

  async function build() {
    const token = ++buildToken;
    await tick();
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));

    if (token !== buildToken || !mounted || !tableComponent) {
      return;
    }

    destroyTable();
    tableComponent.replaceChildren();

    if (data.length === 0) {
      return;
    }

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
  }

  function destroyTable() {
    if (!tabulator) {
      return;
    }

    tabulator.destroy();
    tabulator = undefined;
  }

  onMount(async () => {
    mounted = true;
    build();
  });

  onDestroy(() => {
    mounted = false;
    buildToken += 1;
    destroyTable();
  });
</script>

<div
  class="overflow-x-auto py-0"
  class:box={!preview}
  class:paisa-preview-table={preview}
  style="max-width: 100%;"
  bind:this={tableComponent}
></div>
