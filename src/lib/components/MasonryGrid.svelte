<script lang="ts">
  // @egjs/svelte-grid ships a Svelte 4 wrapper that instantiates its component as a
  // class. Svelte 5 components are functions, so use the underlying Grid component
  // directly instead of the package's MasonryGrid export.
  import RawGridView from "@egjs/svelte-grid/src/Grid.svelte";
  import { MasonryGrid as MasonryGridClass } from "@egjs/grid";

  const GridView = RawGridView as any;
</script>

<GridView GridClass={MasonryGridClass} {...$$props}>
  <slot />
</GridView>
